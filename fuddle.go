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

	conn *grpc.ClientConn

	// cancel is a function called when the client is shutdown to stop any
	// waiting contexts.
	cancelCtx context.Context
	cancel    func()
	wg        sync.WaitGroup

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
		connectAttemptTimeout:   options.connectAttemptTimeout,
		onConnectionStateChange: options.onConnectionStateChange,

		cancelCtx: cancelCtx,
		cancel:    cancel,

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

// Close closes the clients connection to Fuddle and unregisters all members
// registered by this client.
func (f *Fuddle) Close() {
	// Note cancel the conn monitor before closing to avoid getting notified
	// about a disconnect.
	f.cancel()

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
