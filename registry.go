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
	"sync"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
)

type subscriber struct {
	Callback func(nodes []Node)
	Options  []Option
}

type registry struct {
	nodes map[string]*rpc.Node
	// localNodes is a set containing the nodes registered by this client.
	localNodes map[string]interface{}

	subscribers map[*subscriber]interface{}

	// mu protects the above fields.
	mu sync.Mutex
}

func newRegistry() *registry {
	return &registry{
		nodes:       make(map[string]*rpc.Node),
		localNodes:  make(map[string]interface{}),
		subscribers: make(map[*subscriber]interface{}),
	}
}

func (r *registry) Node(id string) (Node, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n, ok := r.nodes[id]; ok {
		return NodeFromRPC(n), true
	}
	return Node{}, false
}

func (r *registry) Nodes(opts ...Option) []Node {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.nodesLocked(opts...)
}

func (r *registry) LocalNodeIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var ids []string
	for id := range r.localNodes {
		ids = append(ids, id)
	}
	return ids
}

func (r *registry) Subscribe(cb func(nodes []Node), opts ...Option) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub := &subscriber{
		Callback: cb,
		Options:  opts,
	}
	r.subscribers[sub] = struct{}{}

	cb(r.nodesLocked(opts...))

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		delete(r.subscribers, sub)
	}
}

func (r *registry) ApplyRemoteUpdate(update *rpc.NodeUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ignore updates about local nodes.
	if _, ok := r.localNodes[update.NodeId]; ok {
		return
	}

	switch update.UpdateType {
	case rpc.NodeUpdateType_REGISTER:
		r.applyRegisterUpdateLocked(update)
	case rpc.NodeUpdateType_UNREGISTER:
		r.applyUnregisterUpdateLocked(update)
	case rpc.NodeUpdateType_METADATA:
		r.applyMetadataUpdateLocked(update)
	}

	r.notifySubscribersLocked()
}

func (r *registry) RegisterLocal(node *rpc.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes[node.Id] = node
	r.localNodes[node.Id] = struct{}{}

	r.notifySubscribersLocked()
}

func (r *registry) UnregisterLocal(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, nodeID)
	delete(r.localNodes, nodeID)

	r.notifySubscribersLocked()
}

func (r *registry) UpdateMetadataLocal(nodeID string, metadata map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		return
	}

	// Versions are ignored for the local node.
	for k, v := range metadata {
		node.Metadata[k] = &rpc.VersionedValue{
			Value: v,
		}
	}
}

func (r *registry) applyRegisterUpdateLocked(update *rpc.NodeUpdate) {
	r.nodes[update.NodeId] = &rpc.Node{
		Id:         update.NodeId,
		Attributes: update.Attributes,
		Metadata:   update.Metadata,
	}
}

func (r *registry) applyUnregisterUpdateLocked(update *rpc.NodeUpdate) {
	delete(r.nodes, update.NodeId)
}

func (r *registry) applyMetadataUpdateLocked(update *rpc.NodeUpdate) {
	node, ok := r.nodes[update.NodeId]
	if !ok {
		return
	}

	for k, vv := range update.Metadata {
		node.Metadata[k] = vv
	}
}

func (r *registry) nodesLocked(opts ...Option) []Node {
	options := &options{}
	for _, o := range opts {
		o.apply(options)
	}

	nodes := make([]Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		n := NodeFromRPC(n)
		if options.filter == nil || options.filter.Match(n) {
			nodes = append(nodes, n.Copy())
		}
	}
	return nodes
}

func (r *registry) notifySubscribersLocked() {
	for sub := range r.subscribers {
		sub.Callback(r.nodesLocked(sub.Options...))
	}
}
