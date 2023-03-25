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

package fuddle

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	clusterAddr = "127.0.0.1:8220"
)

func TestFuddle_RegisterAndReceiveUpdate(t *testing.T) {
	observerConn, err := Connect([]string{clusterAddr})
	require.NoError(t, err)
	defer observerConn.Close()

	registerConn, err := Connect([]string{clusterAddr})
	require.NoError(t, err)
	defer registerConn.Close()

	registeredNode := randomNode()
	n, err := registerConn.Register(context.TODO(), registeredNode)
	assert.NoError(t, err)

	receivedNode, err := waitForNode(observerConn, registeredNode.ID)
	assert.NoError(t, err)

	assert.Equal(t, receivedNode, registeredNode)

	assert.NoError(t, n.Unregister(context.TODO()))

	assert.NoError(t, waitForCount(observerConn, 0))
}

// Tests the client connection will succeed even if some of the seed addresses
// are wrong.
func TestFuddle_ConnectBadSeedAddresses(t *testing.T) {
	addrs := []string{
		// Blocked port.
		"fuddle.io:12345",
		// Bad protocol.
		"fuddle.io:443",
		// No host.
		"notfound.fuddle.io:12345",
		// Correct addr.
		clusterAddr,
	}
	c, err := Connect(addrs, WithConnectTimeout(time.Millisecond*100))
	require.NoError(t, err)
	defer c.Close()
}

// Tests the client connection will fail if all seed addresses are wrong.
func TestFuddle_BadConnection(t *testing.T) {
	addrs := []string{
		// Blocked port.
		"fuddle.io:12345",
	}
	_, err := Connect(addrs, WithConnectTimeout(time.Millisecond*100))
	require.Error(t, err)
}

func TestFuddle_ConnectNoAddresses(t *testing.T) {
	addrs := []string{}
	_, err := Connect(addrs, WithConnectTimeout(time.Millisecond*100))
	require.Error(t, err)
}

func waitForNode(client *Fuddle, id string) (Node, error) {
	var node Node
	found := false
	recvCh := make(chan interface{})
	unsubscribe := client.Subscribe(func() {
		if found {
			return
		}

		for _, n := range client.Nodes() {
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

func waitForCount(client *Fuddle, count int) error {
	found := false
	recvCh := make(chan interface{})
	unsubscribe := client.Subscribe(func() {
		if found {
			return
		}

		if len(client.Nodes()) == count {
			found = true
			close(recvCh)
			return
		}
	})
	defer unsubscribe()

	if err := waitWithTimeout(recvCh, time.Millisecond*500); err != nil {
		return err
	}
	return nil
}

func waitWithTimeout(ch chan interface{}, timeout time.Duration) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	}
}
