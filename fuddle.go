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
	"sync"
	"time"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	multierror "github.com/hashicorp/go-multierror"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Fuddle streams updates to the registry and handles registering nodes into
// the registry.
type Fuddle struct {
	conn   *grpc.ClientConn
	client rpc.RegistryClient

	registry *registry

	closed *atomic.Bool
	wg     sync.WaitGroup

	logger *zap.Logger
}

// Connect connects to one of the given addresses and streams the registry
// state to maintain a local eventually consistent view of the cluster.
//
// The given addresses are a set of seed addresses for Fuddle nodes.
func Connect(addrs []string, opts ...Option) (*Fuddle, error) {
	options := &options{
		logger:         zap.NewNop(),
		connectTimeout: time.Millisecond * 1000,
	}
	for _, o := range opts {
		o.apply(options)
	}

	fuddle := &Fuddle{
		registry: newRegistry(),
		closed:   atomic.NewBool(false),
		logger:   options.logger,
	}

	conn, err := fuddle.rpcConnect(addrs, options.connectTimeout)
	if err != nil {
		return nil, fmt.Errorf("fuddle: %w", err)
	}

	fuddle.conn = conn
	fuddle.client = rpc.NewRegistryClient(conn)

	updateStream, err := fuddle.client.Updates(
		context.Background(), &rpc.UpdatesRequest{},
	)
	if err != nil {
		return nil, fmt.Errorf("fuddle: update stream: %w", err)
	}

	fuddle.wg.Add(1)
	go func() {
		defer fuddle.wg.Done()
		fuddle.streamUpdates(updateStream)
	}()

	return fuddle, nil
}

// Nodes returns the set of nodes in the cluster.
func (f *Fuddle) Nodes(opts ...Option) []Node {
	return f.registry.Nodes(opts...)
}

// Subscribe registers the given callback to fire when the registry state
// changes.
//
// The callback will be called immediately after registering with the current
// node state.
//
// Note the callback is called synchronously with the registry mutex held,
// therefore it must NOT block or callback to the registry (or it will
// deadlock).
func (f *Fuddle) Subscribe(cb func(nodes []Node), opts ...Option) func() {
	return f.registry.Subscribe(cb, opts...)
}

// Register registers the given node and returns a reference to the node so
// it can be updated and unregistered.
func (f *Fuddle) Register(ctx context.Context, node Node) (*LocalNode, error) {
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}

	// Versions only set by the registry so leave as 0.
	versionedMetadata := make(map[string]*rpc.VersionedValue)
	for k, v := range node.Metadata {
		versionedMetadata[k] = &rpc.VersionedValue{
			Value: v,
		}
	}

	req := &rpc.RegisterRequest{
		Node: &rpc.Node{
			Id: node.ID,
			Attributes: &rpc.NodeAttributes{
				Service:  node.Service,
				Locality: node.Locality,
				Created:  node.Created,
				Revision: node.Revision,
			},
			Metadata: versionedMetadata,
		},
	}
	resp, err := f.client.Register(ctx, req)
	if err != nil {
		f.logger.Error(
			"failed to register node",
			zap.String("id", node.ID),
			zap.Error(err),
		)

		return nil, fmt.Errorf("fuddle: register: %w", err)
	}
	if resp.Error != nil {
		f.logger.Error(
			"failed to register node",
			zap.String("id", node.ID),
			zap.String("error", resp.Error.Description),
		)

		return nil, fmt.Errorf("fuddle: register: %s", resp.Error.Description)
	}

	f.logger.Debug("node registered", zap.String("id", node.ID))

	return newLocalNode(node.ID, f.client, f.logger), nil
}

// Close closes the connection to the server and unregisters any registered
// nodes.
//
// Note it is important this is called before the node is shutdown otherwise
// the registry will view all nodes registered by this client as failed instead
// of left.
func (f *Fuddle) Close() {
	f.closed.Store(true)
	f.conn.Close()
	f.wg.Wait()
}

func (f *Fuddle) streamUpdates(stream rpc.Registry_UpdatesClient) {
	for {
		update, err := stream.Recv()
		if err != nil {
			// Check if closed to avoid logging an error when the client closes.
			if !f.closed.Load() {
				f.logger.Error("stream error", zap.Error(err))
			}
			return
		}

		f.logger.Debug(
			"received update",
			zap.String("id", update.NodeId),
			zap.String("update-type", update.UpdateType.String()),
		)

		f.registry.ApplyUpdate(update)
	}
}

func (f *Fuddle) rpcConnect(addrs []string, timeout time.Duration) (*grpc.ClientConn, error) {
	if len(addrs) == 0 {
		f.logger.Error("failed to connect: no addresses")
		return nil, fmt.Errorf("connect: no addresses")
	}

	var result error
	for _, addr := range addrs {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		conn, err := grpc.DialContext(
			ctx,
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			// Block until connected so we know the address is ok.
			grpc.WithBlock(),
		)
		if err != nil {
			f.logger.Warn(
				"failed to connect to seed addr",
				zap.String("addr", addr),
				zap.Error(err),
			)

			result = multierror.Append(result, err)
			continue
		}

		f.logger.Info(
			"connected to fuddle",
			zap.String("addr", addr),
		)

		return conn, nil
	}

	f.logger.Error("failed to connect: all connect attempts failed")

	return nil, fmt.Errorf("connect: %w", result)
}
