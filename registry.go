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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	multierror "github.com/hashicorp/go-multierror"
)

// Registry manages the nodes entry into the cluster registry.
type Registry struct {
	// nodeID is the ID of this registered node.
	nodeID string

	cluster *cluster

	conn *websocket.Conn

	wg sync.WaitGroup
}

// Register registers the given node with the cluster registry.
//
// Once registered the nodes state will be propagated to the other nodes in
// the cluster. It will also stream the existing cluster state and any future
// updates to maintain a local eventually consistent view of the cluster.
//
// The given addresses are a set of seed addresses for Fuddle nodes.
func Register(addrs []string, node Node, opts ...RegistryOption) (*Registry, error) {
	options := &registryOptions{
		connectTimeout: time.Millisecond * 1000,
	}
	for _, o := range opts {
		o.apply(options)
	}

	conn, err := connect(addrs, options.connectTimeout)
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}

	r := &Registry{
		nodeID:  node.ID,
		cluster: newCluster(node),
		conn:    conn,
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
func (r *Registry) Nodes(opts ...NodesOption) []Node {
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
func (r *Registry) Subscribe(cb func(nodes []Node), opts ...NodesOption) func() {
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
		_, b, err := r.conn.ReadMessage()
		if err != nil {
			return
		}

		var update nodeUpdate
		if err := json.Unmarshal(b, &update); err != nil {
			continue
		}
		if err := r.applyUpdate(&update); err != nil {
			return
		}
	}
}

func (r *Registry) sendRegisterUpdate(node Node) error {
	update := &nodeUpdate{
		ID:         node.ID,
		UpdateType: updateTypeRegister,
		Attributes: &nodeAttributes{
			Service:  node.Service,
			Locality: node.Locality,
			Created:  node.Created,
			Revision: node.Revision,
		},
		Metadata: node.Metadata,
	}
	b, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("send register update: encode update: %w", err)
	}
	if err := r.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return fmt.Errorf("send register update: %w", err)
	}
	return nil
}

func (r *Registry) sendMetadataUpdate(metadata map[string]string) error {
	update := &nodeUpdate{
		ID:         r.nodeID,
		UpdateType: updateTypeMetadata,
		Metadata:   metadata,
	}
	b, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("send metadata update: encode update: %w", err)
	}
	if err := r.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return fmt.Errorf("send metadata update: %w", err)
	}
	return nil
}

func (r *Registry) sendUnregisterUpdate() error {
	update := &nodeUpdate{
		ID:         r.nodeID,
		UpdateType: updateTypeUnregister,
	}
	b, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("send unregister update: encode update: %w", err)
	}
	if err := r.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return fmt.Errorf("send unregister update: %w", err)
	}
	return nil
}

func (r *Registry) applyUpdate(update *nodeUpdate) error {
	switch update.UpdateType {
	case updateTypeRegister:
		if err := r.applyRegisterUpdateLocked(update); err != nil {
			return err
		}
	case updateTypeUnregister:
		r.applyUnregisterUpdateLocked(update)
	case updateTypeMetadata:
		if err := r.applyMetadataUpdateLocked(update); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cluster: unknown update type: %s", update.UpdateType)
	}

	return nil
}

func (r *Registry) applyRegisterUpdateLocked(update *nodeUpdate) error {
	if update.ID == "" {
		return fmt.Errorf("cluster: join update: missing id")
	}

	if update.Attributes == nil {
		return fmt.Errorf("cluster: join update: missing attributes")
	}

	node := Node{
		ID:       update.ID,
		Service:  update.Attributes.Service,
		Locality: update.Attributes.Locality,
		Revision: update.Attributes.Revision,
		Created:  update.Attributes.Created,
		// Copy the state to avoid modifying the update. If update.Metadata is
		// nil this returns an empty map.
		Metadata: copyMetadata(update.Metadata),
	}
	return r.cluster.AddNode(node)
}

func (r *Registry) applyUnregisterUpdateLocked(update *nodeUpdate) {
	r.cluster.RemoveNode(update.ID)
}

func (r *Registry) applyMetadataUpdateLocked(update *nodeUpdate) error {
	// If the update is missing state must ignore it.
	if update.Metadata == nil {
		return nil
	}
	return r.cluster.UpdateMetadata(update.ID, update.Metadata)
}

func connect(addrs []string, timeout time.Duration) (*websocket.Conn, error) {
	var result error
	for _, addr := range addrs {
		c, err := connectAddr(addr, timeout)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}

		return c, nil
	}

	return nil, fmt.Errorf("connect: %w", result)
}

func connectAddr(addr string, timeout time.Duration) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := "ws://" + addr + "/api/v1/register"
	c, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	return c, nil
}
