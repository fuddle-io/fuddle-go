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

// Fuddle streams updates to the registry and handles registering nodes into
// the registry.
type Fuddle struct {
}

// Connect connects to one of the given addresses and streams the registry
// state to maintain a local eventually consistent view of the cluster.
//
// The given addresses are a set of seed addresses for Fuddle nodes.
func Connect(addrs []string, opts ...RegistryOption) (*Fuddle, error) {
	return &Fuddle{}, nil
}

// Nodes returns the set of nodes in the cluster.
func (f *Fuddle) Nodes(opts ...NodesOption) []Node {
	return nil
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
func (f *Fuddle) Subscribe(cb func(nodes []Node), opts ...NodesOption) func() {
	return nil
}

// Register registers the given node and returns a reference to the node so
// it can be updated and unregistered.
func (f *Fuddle) Register(node Node) (*LocalNode, error) {
	return nil, nil
}

// Close closes the connection to the server and unregisters any registered
// nodes.
//
// Note it is important this is called before the node is shutdown otherwise
// the registry will view all nodes registered by this client as failed instead
// of left.
func (f *Fuddle) Close() {
}
