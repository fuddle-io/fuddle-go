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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Registry manages the nodes entry into the cluster registry.
type Registry struct {
	// nodeID is the ID of this registered node.
	nodeID string

	cluster *cluster

	conn   *grpc.ClientConn
	stream rpc.Registry_RegisterClient

	wg sync.WaitGroup
}

// Register registers the given node with the cluster registry.
//
// Once registered the nodes state will be propagated to the other nodes in
// the cluster. It will also stream the existing cluster state and any future
// updates to maintain a local eventually consistent view of the cluster.
//
// The given addresses are a set of seed addresses for Fuddle nodes.
func Register(addrs []string, node Node, opts ...Option) (*Registry, error) {
	options := &options{
		connectTimeout: time.Millisecond * 1000,
	}
	for _, o := range opts {
		o.apply(options)
	}

	conn, stream, err := connect(addrs, options.connectTimeout)
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}

	r := &Registry{
		nodeID:  node.ID,
		cluster: newCluster(node),
		conn:    conn,
		stream:  stream,
		wg:      sync.WaitGroup{},
	}
	if err = r.sendRegisterUpdate(node); err != nil {
		r.conn.Close()
		return nil, fmt.Errorf("registry: %w", err)
	}

	r.wg.Add(1)
	go r.sync()

	return r, nil
}

// Nodes returns the set of nodes in the cluster.
func (r *Registry) Nodes(opts ...Option) []Node {
	return r.cluster.Nodes(opts...)
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
func (r *Registry) Subscribe(cb func(nodes []Node), opts ...Option) func() {
	return r.cluster.Subscribe(cb, opts...)
}

// UpdateLocalMetadata will update the state of this node, which will be propagated
// to the other nodes in the cluster.
func (r *Registry) UpdateLocalMetadata(update map[string]string) error {
	if err := r.cluster.UpdateLocalMetadata(update); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	if err := r.sendMetadataUpdate(update); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	return nil
}

// Unregister unregisters the node from the cluster registry.
//
// Note nodes must unregister themselves before shutting down. Otherwise
// Fuddle will think the node failed rather than left.
func (r *Registry) Unregister() error {
	err := r.sendUnregisterUpdate()

	r.conn.Close()
	r.wg.Wait()

	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	return nil
}

func (r *Registry) sync() {
	defer r.wg.Done()

	for {
		m, err := r.stream.Recv()
		if err != nil {
			return
		}

		switch m.MessageType {
		case rpc.MessageType_NODE_UPDATE:
			if m.NodeUpdate == nil {
				continue
			}
			if err := r.applyUpdate(m.NodeUpdate); err != nil {
				continue
			}
		default:
		}
	}
}

func (r *Registry) sendRegisterUpdate(node Node) error {
	metadata := make(map[string]*rpc.VersionedValue)
	for k, v := range node.Metadata {
		metadata[k] = &rpc.VersionedValue{
			Value: v,
		}
	}

	update := &rpc.NodeUpdate{
		NodeId:     node.ID,
		UpdateType: rpc.NodeUpdateType_REGISTER,
		Attributes: &rpc.NodeAttributes{
			Service:  node.Service,
			Locality: node.Locality,
			Created:  node.Created,
			Revision: node.Revision,
		},
		Metadata: metadata,
	}
	m := &rpc.Message{
		MessageType: rpc.MessageType_NODE_UPDATE,
		NodeUpdate:  update,
	}
	if err := r.stream.Send(m); err != nil {
		return fmt.Errorf("send register update: %w", err)
	}
	return nil
}

func (r *Registry) sendMetadataUpdate(metadata map[string]string) error {
	rpcMetadata := make(map[string]*rpc.VersionedValue)
	for k, v := range metadata {
		rpcMetadata[k] = &rpc.VersionedValue{
			Value: v,
		}
	}

	update := &rpc.NodeUpdate{
		NodeId:     r.nodeID,
		UpdateType: rpc.NodeUpdateType_METADATA,
		Metadata:   rpcMetadata,
	}
	m := &rpc.Message{
		MessageType: rpc.MessageType_NODE_UPDATE,
		NodeUpdate:  update,
	}
	if err := r.stream.Send(m); err != nil {
		return fmt.Errorf("send metadata update: %w", err)
	}
	return nil
}

func (r *Registry) sendUnregisterUpdate() error {
	update := &rpc.NodeUpdate{
		NodeId:     r.nodeID,
		UpdateType: rpc.NodeUpdateType_UNREGISTER,
	}
	m := &rpc.Message{
		MessageType: rpc.MessageType_NODE_UPDATE,
		NodeUpdate:  update,
	}
	if err := r.stream.Send(m); err != nil {
		return fmt.Errorf("send unregister update: %w", err)
	}
	return nil
}

func (r *Registry) applyUpdate(update *rpc.NodeUpdate) error {
	switch update.UpdateType {
	case rpc.NodeUpdateType_REGISTER:
		if err := r.applyRegisterUpdateLocked(update); err != nil {
			return err
		}
	case rpc.NodeUpdateType_UNREGISTER:
		r.applyUnregisterUpdateLocked(update)
	case rpc.NodeUpdateType_METADATA:
		if err := r.applyMetadataUpdateLocked(update); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cluster: unknown update type: %s", update.UpdateType)
	}

	return nil
}

func (r *Registry) applyRegisterUpdateLocked(update *rpc.NodeUpdate) error {
	if update.NodeId == "" {
		return fmt.Errorf("cluster: join update: missing id")
	}

	if update.Attributes == nil {
		return fmt.Errorf("cluster: join update: missing attributes")
	}

	metadata := make(map[string]string)
	for k, vv := range update.Metadata {
		metadata[k] = vv.Value
	}
	node := Node{
		ID:       update.NodeId,
		Service:  update.Attributes.Service,
		Locality: update.Attributes.Locality,
		Revision: update.Attributes.Revision,
		Created:  update.Attributes.Created,
		Metadata: metadata,
	}
	return r.cluster.AddNode(node)
}

func (r *Registry) applyUnregisterUpdateLocked(update *rpc.NodeUpdate) {
	r.cluster.RemoveNode(update.NodeId)
}

func (r *Registry) applyMetadataUpdateLocked(update *rpc.NodeUpdate) error {
	// If the update is missing state must ignore it.
	if update.Metadata == nil {
		return nil
	}
	metadata := make(map[string]string)
	for k, vv := range update.Metadata {
		metadata[k] = vv.Value
	}
	return r.cluster.UpdateMetadata(update.NodeId, metadata)
}

func connect(addrs []string, timeout time.Duration) (*grpc.ClientConn, rpc.Registry_RegisterClient, error) {
	var result error
	for _, addr := range addrs {
		conn, stream, err := connectAddr(addr, timeout)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}

		return conn, stream, nil
	}

	return nil, nil, fmt.Errorf("connect: %w", result)
}

func connectAddr(addr string, timeout time.Duration) (*grpc.ClientConn, rpc.Registry_RegisterClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	stream, err := rpc.NewRegistryClient(conn).Register(context.Background())
	if err != nil {
		return nil, nil, err
	}

	return conn, stream, nil
}
