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

	subscribers map[*subscriber]interface{}

	// mu protects the above fields.
	mu sync.Mutex
}

func newRegistry() *registry {
	return &registry{
		nodes:       make(map[string]*rpc.Node),
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

func (r *registry) ApplyUpdate(update *rpc.NodeUpdate) {
	switch update.UpdateType {
	case rpc.NodeUpdateType_REGISTER:
		r.applyRegisterUpdate(update)
	case rpc.NodeUpdateType_UNREGISTER:
		r.applyUnregisterUpdate(update)
	case rpc.NodeUpdateType_METADATA:
		r.applyMetadataUpdate(update)
	}
}

func (r *registry) applyRegisterUpdate(update *rpc.NodeUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes[update.NodeId] = &rpc.Node{
		Id:         update.NodeId,
		Attributes: update.Attributes,
		Metadata:   update.Metadata,
	}

	r.notifySubscribersLocked()
}

func (r *registry) applyUnregisterUpdate(update *rpc.NodeUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, update.NodeId)

	r.notifySubscribersLocked()
}

func (r *registry) applyMetadataUpdate(update *rpc.NodeUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, ok := r.nodes[update.NodeId]
	if !ok {
		return
	}

	for k, vv := range update.Metadata {
		node.Metadata[k] = vv
	}

	r.notifySubscribersLocked()
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
