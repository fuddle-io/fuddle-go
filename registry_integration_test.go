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

//go:build integration

package fuddle_test

import (
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	fuddle "github.com/fuddle-io/fuddle-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	flagCluster = flag.String("cluster", "127.0.0.1:8220", "a comma-separated list of host:port tuples")
)

func getClusterAddrs() []string {
	return strings.Split(*flagCluster, ",")
}

// Note these tests do not assume exclusive use of the Fuddle cluster. If the
// future this can be improved by adding namespaces so each test can have its
// own isolated view of the cluster.

// Tests registering a node can see itself in the registry and the connected
// fuddle node.
func TestRegistry_RegisterNode(t *testing.T) {
	localNode := randomNode()
	registry, err := fuddle.Register(getClusterAddrs(), localNode)
	require.NoError(t, err)
	defer registry.Unregister()

	// Wait to receive a Fuddle server node.
	_, err = waitForService(registry, "fuddle")
	require.NoError(t, err)
	// Verify we are in the set of nodes.
	assert.True(t, nodesContains(registry.Nodes(), localNode))
}

// Tests when a node registers it discovers the existing set of nodes in the
// cluster.
func TestRegistry_DiscoverNodes(t *testing.T) {
	// Register 10 nodes.
	registeredNodes := make(map[string]fuddle.Node)
	for i := 0; i != 10; i++ {
		node := randomNode()
		registeredNodes[node.ID] = node

		registry, err := fuddle.Register(getClusterAddrs(), node)
		require.NoError(t, err)
		defer registry.Unregister()
	}

	registry, err := fuddle.Register(getClusterAddrs(), randomNode())
	require.NoError(t, err)
	defer registry.Unregister()

	// Register another node and verify it has discovered the nodes already
	// in the cluster.
	for id, node := range registeredNodes {
		discoveredNode, err := waitForNode(registry, id)
		assert.NoError(t, err)
		assert.Equal(t, node, discoveredNode)
	}
}

// Tests the registry will attempt multiple addresses.
func TestRegistry_ConnectTriesAllAddresses(t *testing.T) {
	addrs := []string{
		// Address whos firewall blocks the port.
		"fuddle.io:12345",
		"google.com:12345",
		// No host.
		"notfound.fuddle.io:12345",
	}
	addrs = append(addrs, getClusterAddrs()...)

	localNode := randomNode()
	registry, err := fuddle.Register(addrs, localNode)
	require.NoError(t, err)
	defer registry.Unregister()
}

func nodesContains(nodes []fuddle.Node, node fuddle.Node) bool {
	for _, n := range nodes {
		if n.Equal(node) {
			return true
		}
	}
	return false
}

func nodesContainsService(nodes []fuddle.Node, service string) bool {
	for _, n := range nodes {
		if n.Service == service {
			return true
		}
	}
	return false
}

func waitForService(registry *fuddle.Registry, service string) (fuddle.Node, error) {
	var node fuddle.Node
	found := false
	recvCh := make(chan interface{})
	unsubscribe := registry.Subscribe(func(nodes []fuddle.Node) {
		if found {
			return
		}

		for _, n := range nodes {
			if n.Service == service {
				found = true
				node = n
				close(recvCh)
				return
			}
		}
	})
	defer unsubscribe()

	if err := waitWithTimeout(recvCh, time.Millisecond*500); err != nil {
		return node, err
	}
	return node, nil
}

func waitForNode(registry *fuddle.Registry, id string) (fuddle.Node, error) {
	var node fuddle.Node
	found := false
	recvCh := make(chan interface{})
	unsubscribe := registry.Subscribe(func(nodes []fuddle.Node) {
		if found {
			return
		}

		for _, n := range nodes {
			if n.ID == id {
				found = true
				node = n
				close(recvCh)
				return
			}
		}
	})
	defer unsubscribe()

	if err := waitWithTimeout(recvCh, time.Millisecond*500); err != nil {
		return node, err
	}
	return node, nil
}

func waitForNodes(registry *fuddle.Registry, count int) ([]fuddle.Node, error) {
	recvCh := make(chan interface{})
	found := false
	var nodes []fuddle.Node
	unsubscribe := registry.Subscribe(func(n []fuddle.Node) {
		if found {
			return
		}

		if len(n) == count {
			found = true
			nodes = n
			close(recvCh)
			return
		}
	})
	defer unsubscribe()

	if err := waitWithTimeout(recvCh, time.Millisecond*500); err != nil {
		return nil, err
	}
	return nodes, nil
}

func waitWithTimeout(ch chan interface{}, timeout time.Duration) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	}
}

// randomNode returns a node with random attributes and state.
func randomNode() fuddle.Node {
	return fuddle.Node{
		ID:       uuid.New().String(),
		Service:  uuid.New().String(),
		Locality: uuid.New().String(),
		Created:  rand.Int63(),
		Revision: uuid.New().String(),
		Metadata: map[string]string{
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
		},
	}
}
