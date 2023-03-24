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
	"math/rand"
	"sort"
	"testing"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRegistry_RegisterThenQueryNode(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.ApplyRemoteUpdate(nodeToRegisterUpdate(registeredNode))

	n, ok := r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)
}

func TestRegistry_RegisterThenUnregister(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.ApplyRemoteUpdate(nodeToRegisterUpdate(registeredNode))
	r.ApplyRemoteUpdate(nodeToUnregisterUpdate(registeredNode))

	_, ok := r.Node(registeredNode.ID)
	assert.False(t, ok)
}

func TestRegistry_NodeNotFound(t *testing.T) {
	r := newRegistry()

	_, ok := r.Node("foo")
	assert.False(t, ok)
}

func TestRegistry_UpdateNodeMetadata(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.ApplyRemoteUpdate(nodeToRegisterUpdate(registeredNode))

	update := randomMetadata()
	r.ApplyRemoteUpdate(metadataToMetadataUpdate(registeredNode.ID, update))

	expectedNode := registeredNode
	for k, v := range update {
		expectedNode.Metadata[k] = v
	}

	n, ok := r.Node(expectedNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, expectedNode)
}

func TestRegistry_NodesLookupWithFilter(t *testing.T) {
	tests := []struct {
		Filter        Filter
		AddedNodes    []Node
		FilteredNodes []Node
	}{
		// Filter no nodes.
		{
			Filter: Filter{
				"service-1": {
					Locality: []string{"eu-west-1-*", "eu-west-2-*"},
					Metadata: MetadataFilter{
						"foo": []string{"bar", "car", "boo"},
					},
				},
				"service-2": {
					Locality: []string{"us-east-1-*", "eu-west-2-*"},
					Metadata: MetadataFilter{
						"bar": []string{"bar", "car", "boo"},
					},
				},
			},
			AddedNodes: []Node{
				Node{
					ID:       "1",
					Service:  "service-1",
					Locality: "eu-west-2-c",
					Metadata: map[string]string{
						"foo": "car",
					},
				},
				Node{
					ID:       "2",
					Service:  "service-2",
					Locality: "us-east-1-c",
					Metadata: map[string]string{
						"bar": "boo",
					},
				},
			},
			FilteredNodes: []Node{
				Node{
					ID:       "1",
					Service:  "service-1",
					Locality: "eu-west-2-c",
					Metadata: map[string]string{
						"foo": "car",
					},
				},
				Node{
					ID:       "2",
					Service:  "service-2",
					Locality: "us-east-1-c",
					Metadata: map[string]string{
						"bar": "boo",
					},
				},
			},
		},

		// Filter partial nodes.
		{
			Filter: Filter{
				"service-1": {
					Locality: []string{"eu-west-1-*", "eu-west-2-*"},
					Metadata: MetadataFilter{
						"foo": []string{"bar", "car", "boo"},
					},
				},
			},
			AddedNodes: []Node{
				Node{
					ID:       "1",
					Service:  "service-1",
					Locality: "eu-west-2-c",
					Metadata: map[string]string{
						"foo": "car",
					},
				},
				Node{
					ID:       "2",
					Service:  "service-2",
					Locality: "us-east-1-c",
					Metadata: map[string]string{
						"bar": "boo",
					},
				},
			},
			FilteredNodes: []Node{
				Node{
					ID:       "1",
					Service:  "service-1",
					Locality: "eu-west-2-c",
					Metadata: map[string]string{
						"foo": "car",
					},
				},
			},
		},

		// Filter all nodes.
		{
			Filter: Filter{},
			AddedNodes: []Node{
				Node{
					ID:       "1",
					Service:  "service-1",
					Locality: "eu-west-2-c",
					Metadata: map[string]string{
						"foo": "car",
					},
				},
				Node{
					ID:       "2",
					Service:  "service-2",
					Locality: "us-east-1-c",
					Metadata: map[string]string{
						"bar": "boo",
					},
				},
			},
			FilteredNodes: []Node{},
		},
	}

	for _, tt := range tests {
		r := newRegistry()
		for _, n := range tt.AddedNodes {
			r.ApplyRemoteUpdate(nodeToRegisterUpdate(n))
		}

		nodes := r.Nodes(WithFilter(tt.Filter))
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
		assert.Equal(t, tt.FilteredNodes, nodes)
	}
}

func TestRegistry_RegisterLocal(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.RegisterLocal(registeredNode.ToRPCNode())

	n, ok := r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)

	assert.Equal(t, []string{registeredNode.ID}, r.LocalNodeIDs())
}

func TestRegistry_UnregisterLocal(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.RegisterLocal(registeredNode.ToRPCNode())
	r.UnregisterLocal(registeredNode.ID)

	_, ok := r.Node(registeredNode.ID)
	assert.False(t, ok)

	assert.Equal(t, []string(nil), r.LocalNodeIDs())
}

func TestRegistry_UpdateMetadataLocal(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.RegisterLocal(registeredNode.ToRPCNode())

	update := randomMetadata()
	r.UpdateMetadataLocal(registeredNode.ID, update)

	expectedNode := registeredNode
	for k, v := range update {
		expectedNode.Metadata[k] = v
	}

	n, ok := r.Node(expectedNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, expectedNode)
}

func TestRegistry_RemoteUpdatesToLocalNodesIgnored(t *testing.T) {
	r := newRegistry()

	registeredNode := randomNode()
	r.RegisterLocal(registeredNode.ToRPCNode())

	n, ok := r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)

	// Attempt to register another node with the same ID, which should be
	// ignored.
	duplicateNode := randomNode()
	duplicateNode.ID = registeredNode.ID
	r.ApplyRemoteUpdate(nodeToRegisterUpdate(duplicateNode))

	// The node should not have been updated.
	n, ok = r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)

	// Attempt to update the node metadata, which should be ignored.
	update := randomMetadata()
	r.ApplyRemoteUpdate(metadataToMetadataUpdate(registeredNode.ID, update))

	// The node should not have been updated.
	n, ok = r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)

	// Attempt to unregister the node, which should be ignored.
	r.ApplyRemoteUpdate(nodeToUnregisterUpdate(registeredNode))

	// The node should not have been updated.
	n, ok = r.Node(registeredNode.ID)
	assert.True(t, ok)
	assert.Equal(t, n, registeredNode)
}

func randomNode() Node {
	return Node{
		ID:       uuid.New().String(),
		Service:  uuid.New().String(),
		Locality: uuid.New().String(),
		Created:  rand.Int63(),
		Revision: uuid.New().String(),
		Metadata: randomMetadata(),
	}
}

func randomMetadata() map[string]string {
	return map[string]string{
		uuid.New().String(): uuid.New().String(),
		uuid.New().String(): uuid.New().String(),
		uuid.New().String(): uuid.New().String(),
		uuid.New().String(): uuid.New().String(),
		uuid.New().String(): uuid.New().String(),
	}
}

func nodeToRegisterUpdate(node Node) *rpc.NodeUpdate {
	metadata := make(map[string]*rpc.VersionedValue)
	for k, v := range node.Metadata {
		metadata[k] = &rpc.VersionedValue{
			Value: v,
		}
	}

	return &rpc.NodeUpdate{
		NodeId:     node.ID,
		UpdateType: rpc.NodeUpdateType_REGISTER,
		Version:    0,
		Attributes: &rpc.NodeAttributes{
			Service:  node.Service,
			Locality: node.Locality,
			Created:  node.Created,
			Revision: node.Revision,
		},
		Metadata: metadata,
	}
}

func nodeToUnregisterUpdate(node Node) *rpc.NodeUpdate {
	return &rpc.NodeUpdate{
		NodeId:     node.ID,
		UpdateType: rpc.NodeUpdateType_UNREGISTER,
	}
}

func metadataToMetadataUpdate(nodeID string, metadata map[string]string) *rpc.NodeUpdate {
	update := make(map[string]*rpc.VersionedValue)
	for k, v := range metadata {
		update[k] = &rpc.VersionedValue{
			Value: v,
		}
	}

	return &rpc.NodeUpdate{
		NodeId:     nodeID,
		UpdateType: rpc.NodeUpdateType_METADATA,
		Metadata:   update,
	}
}
