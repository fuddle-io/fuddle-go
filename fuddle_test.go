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

package fuddle_test

import (
	"context"
	"fmt"
	"time"

	fuddle "github.com/fuddle-io/fuddle-go"
)

// Registers an 'orders' service node in 'us-east-1-b'.
func Example_registerOrdersServiceNode() {
	client, err := fuddle.Connect(
		// Seed addresses of Fuddle servers.
		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	)
	if err != nil {
		// handle err ...
	}
	defer client.Close()

	node, err := client.Register(context.TODO(), fuddle.Node{
		ID:       "orders-32eaba4e",
		Service:  "orders",
		Locality: "aws.us-east-1-b",
		Created:  time.Now().UnixMilli(),
		Revision: "v5.1.0-812ebbc",
		Metadata: map[string]string{
			"status":           "booting",
			"addr.rpc.ip":      "192.168.2.1",
			"addr.rpc.port":    "5562",
			"addr.admin.ip":    "192.168.2.1",
			"addr.admin.port":  "7723",
			"protocol.version": "3",
			"instance":         "i-0bc636e38d6c537a7",
		},
	})
	if err != nil {
		// handle err ...
	}
	defer node.Unregister(context.TODO())

	// ...

	// Once ready update the nodes status to 'active'. This update will be
	// propagated to the other nodes in the cluster.
	err = node.UpdateMetadata(context.TODO(), map[string]string{
		"status": "active",
	})
	if err != nil {
		// handle err ...
	}
}

// Queries the set of active order service nodes in us-east-1.
func Example_lookupOrdersServiceNodes() {
	client, err := fuddle.Connect(
		// Seed addresses of Fuddle servers.
		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	)
	if err != nil {
		// handle err ...
	}
	defer client.Close()

	// ...

	// Filter to only include order service nodes in us-east-1 whose status
	// is active and protocol version is either 2 or 3.
	orderNodes := client.Nodes(fuddle.WithFilter(fuddle.Filter{
		"order": {
			Locality: []string{"aws.us-east-1-*"},
			Metadata: fuddle.MetadataFilter{
				"status":           []string{"active"},
				"protocol.version": []string{"2", "3"},
			},
		},
	}))
	addrs := []string{}
	for _, node := range orderNodes {
		addr := node.Metadata["addr.rpc.ip"] + ":" + node.Metadata["addr.rpc.port"]
		addrs = append(addrs, addr)
	}

	// ...

	fmt.Println("order service:", addrs)
}

// Subscribes to the set of active order service nodes in us-east-1.
func Example_subscribeToOrdersServiceNodes() {
	client, err := fuddle.Connect(
		// Seed addresses of Fuddle servers.
		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	)
	if err != nil {
		// handle err ...
	}
	defer client.Close()

	filter := fuddle.Filter{
		"order": {
			Locality: []string{"aws.us-east-1-*"},
			Metadata: fuddle.MetadataFilter{
				"status":           []string{"active"},
				"protocol.version": []string{"2", "3"},
			},
		},
	}

	// Filter to only include order service nodes in us-east-1 whose status
	// is active and protocol version is either 2 or 3.
	var addrs []string
	client.Subscribe(func() {
		addrs = nil
		for _, node := range client.Nodes(fuddle.WithFilter(filter)) {
			addr := node.Metadata["addr.rpc.ip"] + ":" + node.Metadata["addr.rpc.port"]
			addrs = append(addrs, addr)
		}

		fmt.Println("order service:", addrs)
	})
}
