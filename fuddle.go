package fuddle

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/fuddle-io/fuddle-go/internal/resolvers"
	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
)

// Fuddle is a client for Fuddle registry. It streams updates to build a local
// eventually consistent view of the cluster, and registers its own local
// member.
type Fuddle struct {
	connectAttemptTimeout time.Duration
	keepAlivePingInterval time.Duration
	keepAlivePingTimeout  time.Duration
	heartbeatInterval     time.Duration

	onConnectionStateChange func(state ConnState)

	registry *registry

	conn        *grpc.ClientConn
	readClient  rpc.ClientReadRegistryClient
	writeClient rpc.ClientWriteRegistryClient

	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
	closed *atomic.Bool

	logger              *zap.Logger
	grpcLoggerVerbosity int
}

// Connect connects to the registry and registers the given member.
//
// addrs is a list of seed addresses of known Fuddle nodes.
func Connect(ctx context.Context, member Member, addrs []string, opts ...Option) (*Fuddle, error) {
	options := defaultOptions()
	for _, o := range opts {
		o.apply(options)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	f := &Fuddle{
		connectAttemptTimeout: options.connectAttemptTimeout,
		keepAlivePingInterval: options.keepAlivePingInterval,
		keepAlivePingTimeout:  options.keepAlivePingTimeout,
		heartbeatInterval:     options.heartbeatInterval,

		onConnectionStateChange: options.onConnectionStateChange,

		registry: newRegistry(member, options.logger),

		ctx:    cancelCtx,
		cancel: cancel,
		closed: atomic.NewBool(false),

		logger:              options.logger,
		grpcLoggerVerbosity: options.grpcLoggerVerbosity,
	}
	if err := f.connect(ctx, addrs); err != nil {
		return nil, fmt.Errorf("fuddle: %w", err)
	}

	return f, nil
}

// Members returns all known members in the registry.
func (f *Fuddle) Members() []Member {
	return f.registry.Members()
}

// Subscribe subscribes to updates when the registry changes. This also fires
// the callback immediately after subscribing to bootstrap (which avoids having
// to first call Fuddoe.Members).
func (f *Fuddle) Subscribe(cb func()) func() {
	return f.registry.Subscribe(cb)
}

func (f *Fuddle) Close() {
	f.closed.Store(true)
	f.cancel()
	// Note must wait for all goroutines to stop before closing the connection
	// since we unregister before exiting.
	f.wg.Wait()
	f.conn.Close()
}

func (f *Fuddle) connect(ctx context.Context, addrs []string) error {
	if f.grpcLoggerVerbosity > 0 {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(
			os.Stderr, os.Stderr, os.Stderr, f.grpcLoggerVerbosity,
		))
	}

	if len(addrs) == 0 {
		f.logger.Error("failed to connect: no seed addresses")
		return fmt.Errorf("connect: no seeds addresses")
	}

	// Since we use a 'first pick' load balancer, shuffle the addrs so multiple
	// clients with the same addrs don't all try the same node.
	shuffleStrings(addrs)

	f.logger.Info("connecting", zap.Strings("addrs", addrs))

	// Send keep alive pings to detect unresponsive connections and trigger
	// a reconnect.
	keepAliveParams := keepalive.ClientParameters{
		Time:                f.keepAlivePingInterval,
		Timeout:             f.keepAlivePingTimeout,
		PermitWithoutStream: true,
	}
	conn, err := grpc.DialContext(
		ctx,
		// Use the static resolver which uses the configured seed addresses.
		"static:///fuddle",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithResolvers(resolvers.NewStaticResolverBuilder(addrs)),
		// Add a custom dialer so we can set a per connection attempt timeout.
		grpc.WithContextDialer(f.dialerWithTimeout),
		// Block until the connection succeeds so we can fail the initial
		// connection.
		grpc.WithBlock(),
		grpc.WithKeepaliveParams(keepAliveParams),
	)
	if err != nil {
		f.logger.Error(
			"failed to connect",
			zap.Strings("seeds", addrs),
			zap.Error(err),
		)
		return fmt.Errorf("connect: %w", err)
	}

	f.conn = conn
	f.readClient = rpc.NewClientReadRegistryClient(conn)
	f.writeClient = rpc.NewClientWriteRegistryClient(conn)

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.monitorConnection()
	}()

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

		if !f.conn.WaitForStateChange(f.ctx, s) {
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

	f.setupStreamUpdates()
	f.setupStreamRegister()
}

func (f *Fuddle) onDisconnect() {
	f.logger.Info("disconnected")

	if f.onConnectionStateChange != nil {
		f.onConnectionStateChange(StateDisconnected)
	}
}

func (f *Fuddle) setupStreamUpdates() {
	subscription, err := f.readClient.Updates(
		f.ctx,
		&rpc.SubscribeRequest{
			KnownMembers: f.registry.KnownVersions(),
			// Receive all updates from the connected node..
			OwnerOnly: false,
		},
	)
	if err != nil {
		// If we can't subscribe, this will typically mean we've disconnected
		// so will retry once reconnected.
		f.logger.Warn("failed to subscribe", zap.Error(err))
		return
	}

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.streamUpdates(subscription)
	}()
}

func (f *Fuddle) setupStreamRegister() {
	stream, err := f.writeClient.Register(
		// Use background since f.ctx will be cancelled before we've sent
		// unregister.
		context.Background(),
	)
	if err != nil {
		// If we can't subscribe, this will typically mean we've disconnected
		// so will retry once reconnected.
		f.logger.Warn("failed to stream register", zap.Error(err))
		return
	}

	if err := stream.Send(&rpc.ClientUpdate{
		UpdateType: rpc.ClientUpdateType_CLIENT_REGISTER,
		Member:     f.registry.LocalRPCMember(),
	}); err != nil {
		f.logger.Warn("failed to send register", zap.Error(err))
		return
	}

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.streamHeartbeats(stream)
	}()
}

func (f *Fuddle) streamUpdates(stream rpc.ClientReadRegistry_UpdatesClient) {
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

		f.registry.RemoteUpdate(update)
	}
}

func (f *Fuddle) streamHeartbeats(stream rpc.ClientWriteRegistry_RegisterClient) {
	ticker := time.NewTicker(f.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			if err := stream.Send(&rpc.ClientUpdate{
				UpdateType: rpc.ClientUpdateType_CLIENT_UNREGISTER,
				Member:     f.registry.LocalRPCMember(),
			}); err != nil {
				f.logger.Warn("unregister error", zap.Error(err))
			}
			return
		case <-ticker.C:
			if err := stream.Send(&rpc.ClientUpdate{
				UpdateType: rpc.ClientUpdateType_CLIENT_HEARTBEAT,
			}); err != nil {
				return
			}
		}
	}
}

func (f *Fuddle) dialerWithTimeout(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: f.connectAttemptTimeout,
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

func shuffleStrings(s []string) {
	for i := range s {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
