// Copyright (C) 2023 Andrew Dunstall
//
// Registry is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Registry is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package fuddle

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/fuddle-io/fuddle-gov3/internal/resolvers"
	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"github.com/google/uuid"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

// Fuddle is a client for the Fuddle registry which can be used to subscribe to
// registry updates and register members.
type Fuddle struct {
	connectAttemptTimeout time.Duration

	onConnectionStateChange func(state ConnState)

	clientID string
	registry *registry

	conn   *grpc.ClientConn
	client rpc.RegistryV2Client

	// cancel is a function called when the client is shutdown to stop any
	// waiting contexts.
	cancelCtx context.Context
	cancel    func()
	wg        sync.WaitGroup
	closed    *atomic.Bool

	logger              *zap.Logger
	grpcLoggerVerbosity int
}

// Connect connects to the Fuddle registry and starts streaming registry
// updates.
//
// The seed addresses are addresses of Fuddle nodes in the cluster.
//
// Returns an error if the client fails to connect to a Fuddle node before the
// given context is cancelled.
func Connect(ctx context.Context, seeds []string, opts ...Option) (*Fuddle, error) {
	options := defaultOptions()
	for _, o := range opts {
		o.apply(options)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	f := &Fuddle{
		connectAttemptTimeout: options.connectAttemptTimeout,

		onConnectionStateChange: options.onConnectionStateChange,

		clientID: uuid.New().String(),
		registry: newRegistry(),

		cancelCtx: cancelCtx,
		cancel:    cancel,
		closed:    atomic.NewBool(false),

		logger:              options.logger,
		grpcLoggerVerbosity: options.grpcLoggerVerbosity,
	}

	if err := f.connect(ctx, seeds); err != nil {
		return nil, fmt.Errorf("fuddle: %w", err)
	}

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.monitorConnection()
	}()

	return f, nil
}

func (f *Fuddle) Members() []Member {
	return f.registry.Members()
}

func (f *Fuddle) Subscribe(cb func()) func() {
	return f.registry.Subscribe(cb)
}

func (f *Fuddle) Register(ctx context.Context, member Member) error {
	if member.Metadata == nil {
		member.Metadata = make(map[string]string)
	}

	resp, err := f.client.RegisterMember(ctx, &rpc.RegisterMemberRequest{
		Member: member.toRPC(f.clientID),
	})
	if err != nil {
		f.logger.Error(
			"failed to register member",
			zap.String("id", member.ID),
			zap.Error(err),
		)
		return fmt.Errorf("fuddle: register: %w", err)
	}
	if resp.Error != nil {
		err = rpcErrorToError(resp.Error)
		f.logger.Error(
			"failed to register member",
			zap.String("id", member.ID),
			zap.Error(err),
		)
		return fmt.Errorf("fuddle: register: %w", err)
	}

	f.registry.RegisterLocal(member.toRPC(f.clientID))

	f.logger.Debug("member registered", zap.String("id", member.ID))

	return nil
}

// Close closes the clients connection to Fuddle and unregisters all members
// registered by this client.
func (f *Fuddle) Close() {
	// Note cancel the conn monitor before closing to avoid getting notified
	// about a disconnect.
	f.cancel()
	f.closed.Store(true)

	f.conn.Close()
	f.wg.Wait()
}

func (f *Fuddle) connect(ctx context.Context, seeds []string) error {
	if f.grpcLoggerVerbosity > 0 {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(
			os.Stderr, os.Stderr, os.Stderr, f.grpcLoggerVerbosity,
		))
	}

	if len(seeds) == 0 {
		f.logger.Error("failed to connect: no seed addresses")
		return fmt.Errorf("connect: no seeds addresses")
	}

	// Since we use a 'first pick' load balancer, shuffle the seeds so multiple
	// clients with the same seeds don't all try the same node.
	for i := range seeds {
		j := rand.Intn(i + 1)
		seeds[i], seeds[j] = seeds[j], seeds[i]
	}

	f.logger.Info("connecting", zap.Strings("seeds", seeds))

	conn, err := grpc.DialContext(
		ctx,
		// Use the status resolver which uses the configured seed addresses.
		"static:///fuddle",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithResolvers(resolvers.NewStaticResolverBuilder(seeds)),
		// Add a custom dialer so we can set a per connection attempt timeout.
		grpc.WithContextDialer(f.dialerWithTimeout),
		// Block until the connection succeeds.
		grpc.WithBlock(),
	)
	if err != nil {
		f.logger.Error(
			"failed to connect",
			zap.Strings("seeds", seeds),
			zap.Error(err),
		)
		return fmt.Errorf("connect: %w", err)
	}

	f.conn = conn
	f.client = rpc.NewRegistryV2Client(conn)

	return nil
}

// monitorConnection detects disconnects and reconnects.
func (f *Fuddle) monitorConnection() {
	for {
		s := f.conn.GetState()
		if s == connectivity.Ready {
			f.onConnected()
		} else {
			f.conn.Connect()
		}

		if !f.conn.WaitForStateChange(f.cancelCtx, s) {
			// Only returns if the client is closed.
			return
		}

		// If we were ready but now the state has changed we must have
		// droped the connection.
		if s == connectivity.Ready {
			f.onDisconnect()
		}
	}
}

func (f *Fuddle) onConnected() {
	f.logger.Info("connected")

	if f.onConnectionStateChange != nil {
		f.onConnectionStateChange(StateConnected)
	}

	f.reenterLocalMembers(context.Background())

	subscribeStream, err := f.client.Subscribe(
		context.Background(), &rpc.SubscribeRequest{
			Versions: f.registry.KnownVersions(),
		},
	)
	if err != nil {
		f.logger.Warn("create stream subscribe error", zap.Error(err))
	} else {
		// Start streaming updates. If the connection closes streamUpdates will
		// exit.
		go f.streamUpdates(subscribeStream)
	}
}

func (f *Fuddle) onDisconnect() {
	f.logger.Info("disconnected")

	if f.onConnectionStateChange != nil {
		f.onConnectionStateChange(StateDisconnected)
	}
}

func (f *Fuddle) dialerWithTimeout(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: f.connectAttemptTimeout,
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

func (f *Fuddle) reenterLocalMembers(ctx context.Context) {
	f.logger.Debug(
		"reregistering members",
		zap.Strings("members", f.registry.LocalMemberIDs()),
	)

	for _, member := range f.registry.LocalMembers() {
		resp, err := f.client.RegisterMember(ctx, &rpc.RegisterMemberRequest{
			Member: member.toRPC(f.clientID),
		})
		if err != nil {
			f.logger.Error(
				"failed to reregister member",
				zap.String("id", member.ID),
				zap.Error(err),
			)
			return
		}
		if resp.Error != nil {
			err = rpcErrorToError(resp.Error)
			f.logger.Error(
				"failed to reregister member",
				zap.String("id", member.ID),
				zap.Error(err),
			)
			return
		}

		f.logger.Debug("member re-registered", zap.String("id", member.ID))
	}
}

func (f *Fuddle) streamUpdates(stream rpc.RegistryV2_SubscribeClient) {
	for {
		update, err := stream.Recv()
		if err != nil {
			// Avoid redundent logs if we've closed.
			if f.closed.Load() {
				return
			}
			f.logger.Warn("subscribe error", zap.Error(err))
			return
		}

		f.logger.Debug(
			"received update",
			zap.String("id", update.Id),
			zap.String("update-type", update.UpdateType.String()),
		)

		f.registry.ApplyRemoteUpdate(update)
	}
}

func rpcErrorToError(e *rpc.ErrorV2) error {
	return fmt.Errorf("%s: %s", e.Status, e.Description)
}
