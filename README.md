<p align="center">
  <img src='assets/images/logo.png?raw=true' width='60%'>
</p>


# Fuddle Go

A Go SDK for the [Fuddle](https://github.com/fuddle-io/fuddle) service registry,
which lets nodes register themselves and discover other nodes in the cluster.

---

> :warning: **Fuddle is still in development**

---

# Usage

## Installation
```
go get github.com/fuddle-io/fuddle-go
```

## Register
To enter the Fuddle registry, nodes must register themselves using `Register`.

Once registered, the client loads the state of the cluster and receives updates
when the cluster changes. Therefore the client maintains an eventually
consistent view of the cluster, which the node can query without having to make
RPCs back to the Fuddle node.

The registered node contains:

### Node ID
A unique identifier for the node in the cluster.

### Service
The type of service running on the node. The service is used for service
discovery, such as looking up all nodes in the `foo` service.

### Locality
The location of the node in the cluster. In a cloud environment this will
typically be the availability zone.

The locality is used for service discovery, for example if the locality is the
availability zone, nodes can lookup all nodes in `us-west-1-a`, or all nodes in
a region using a wildcard, such as `us-west-1-*`.

### Created
The UNIX timestamp in milliseconds that the node was created.

### Revision
An identifier for the version of service running on the node, such as a Git tag
or commit.

### State
An arbitrary set of application-defined key-value pairs containing information
about the node that should be propagated to the other nodes in the cluster.

Such as network addresses, protocol versions, lifecycle status etc.

### Example
For example, say you’re building a shopping site, you could register an `orders`
service node in `us-east-1-b`:

```go
registry, err := fuddle.Register(
	// Seed addresses of Fuddle nodes.
	[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	fuddle.Node{
		ID:       "orders-32eaba4e",
		Service:  "orders",
		Locality: "aws.us-east-1-b",
		Created:  time.Now().UnixMilli(),
		Revision: "v5.1.0-812ebbc",
		State: map[string]string{
			"status":           "booting",
			"addr.rpc.ip":      "192.168.2.1",
			"addr.rpc.port":    "5562",
			"addr.admin.ip":    "192.168.2.1",
			"addr.admin.port":  "7723",
			"protocol.version": "3",
			"instance":         "i-0bc636e38d6c537a7",
		},
	},
)
if err != nil {
	// handle err ...
}
```

## Unregister
When a node is shutdown, it must first unregister from the Fuddle registry.
Otherwise Fuddle will view the node as failed rather than having left the
cluster.

Nodes unregister with `registry.Unregister`.

## Service Discovery
Each registered client maintains an eventually consistent view of the cluster.
So nodes can lookup nodes in the cluster and subscribe to updates when the set
of nodes changes, either when nodes join and leave, or when nodes update their
state.

The set of nodes can be filtered based on service, locality and state.

### Lookup
For example, say you want to lookup all the active `order` service nodes in
`us-east-1`:

```go
// Filter to only include order service nodes in us-east-1 whose status
// is active and the protocol version is either 2 or 3.
filter := fuddle.Filter{
	"order": {
		Locality: []string{"aws.us-east-1-*"},
		State: StateFilter{
			"status":           []string{"active"},
			"protocol.version": []string{"2", "3"},
		},
	},
}

orderNodes := registry.Nodes(fuddle.WithFilter(filter))
addrs := []string{}
for _, node := range orderNodes {
	addr := node.State["addr.rpc.ip"] + ":" + node.State["addr.rpc.port"]
	addrs = append(addrs, addr)
}
```


### Subscribe
Alternatively, rather than lookup the nodes, you can subscribe to updates when
nodes join, leave or update their state:

```go
registry.Subscribe(func(orderNodes []fuddle.Node) {
	var addrs []string
	for _, node := range orderNodes {
		addr := node.State["addr.rpc.ip"] + ":" + node.State["addr.rpc.port"]
		addrs = append(addrs, addr)
	}

	// ...
}, WithFilter(filter))
```

# :warning: Limitations
Fuddle is still in early stages of development so has a number of limitations.

See [Fuddle](https://github.com/fuddle-io/fuddle) for details.
