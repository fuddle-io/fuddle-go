// Copyright (C) 2023 Andrew Dunstall
//
// Fuddle is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Fuddle is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package fuddle implements an SDK client for the Fuddle service registry.
//
// Clients use the SDK to register themselves with Fuddle and discover nodes and
// services in the cluster.
//
// # Register
//
// Nodes register by passing their registered node state and a list of Fuddle
// node seed addresses to Register:
//
//	registry, err := fuddle.Register(
//		// List of Fuddle seed addresses.
//		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
//		fuddle.Node{
//			// Attributes describing the local node.
//			ID:       "orders-32eaba4e",
//			Service:  "orders",
//			Locality: "aws.us-east-1-b",
//			Created:  time.Now().UnixMilli(),
//			Revision: "v5.1.0-812ebbc",
//
//			// Application defined metadata to share as part of service discovery.
//			Metadata: map[string]string{
//				"status":           "booting",
//				"addr.rpc.ip":      "192.168.2.1",
//				"addr.rpc.port":    "5562",
//				"protocol.version": "3",
//			},
//		},
//	)
//
// This will register the local node so it can be discovered by other nodes in
// the cluster.
//
// The node state includes a set of attributes that can be used for service
// discovery, such as looking up the nodes in the order service in us-east-1,
// and observability, such as checking the revision and start time of nodes that
// are unhealthy. The attributes are immutable so cannot be changed during the
// lifetime of the node.
//
// Nodes also include a map of application defined metadata which is shared with
// other nodes in the cluster. Such as routing information and protocol version.
// This metadata may be updated using registry.UpdateLocalMetadata, and the
// update will be propagated to the other nodes.
//
// Remember to unregister the node with registry.Unregister() when it is
// shutdown to gracefully leave the cluster. Otherwise Fuddle will consider the
// node as failed rather than unregistered.
//
// # Cluster Discovery
//
// Once a node has registered, it will receive the state of the other nodes in
// the cluster and stream updates about changes to the nodes in the registry.
// Therefore it maintains its own eventually-consistent view of the cluster.
//
// This cluster state can be queried to receive the set of nodes matching some
// filter. Users can also subscribe to updates when the registry is updated, due
// to nodes joining, leaving or updating their metadata.
//
// Lookup a set of nodes:
//
//	registry.Nodes(opts)
//
// Subscribe to changes in a set of nodes:
//
//	registry.Subscribe(callback, opts)
//
// Note when subscribing the callback will fire immediately with the matching
// cluster state, so theres no need to call Nodes first.
//
// # Filters
//
// Queries and subscriptions can filter the set of nodes they are interested in
// based on service name, locality and metadata.
//
// The service, locality and metadata field formats are all user defined,
// however it is recommended to structure as a hierarchy with some delimiter
// like a dot to make it easy to filter using wildcards.
//
// Such as using a format '<provider>.<availability zone>' for the locality lets
// you filter either availability zones ('aws.us-east-1-a'), regions
// ('aws.us-east-1-*') or location ('aws.eu-*').
//
// Wildcards can be used for the service name, locality and metadata values
// (though not metadata keys). The locality and metadata filters also support
// multiple possible values.
//
// For example to filter only order service nodes in us-east-1 whose status
// is 'active' and protocol version is either 2 or 3:
//
//	filter := fuddle.Filter{
//		"order": {
//			Locality: []string{"aws.us-east-1-*"},
//			Metadata: fuddle.MetadataFilter{
//				"status":           []string{"active"},
//				"protocol.version": []string{"2", "3"},
//			},
//		},
//	}
//
//	// Lookup the set of nodes matching the filter.
//	registry.Nodes(fuddle.WithFilter(filter))
//
//	// Subscribe to updates in the set of nodes matching the filter.
//	registry.Subscribe(callback, fuddle.WithFilter(filter))
package fuddle
