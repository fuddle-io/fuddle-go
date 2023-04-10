package fuddle_test

import (
	"context"
	"log"
	"strings"
	"time"

	fuddle "github.com/fuddle-io/fuddle-go"
)

type myLoadBalancer struct {
}

func (lb *myLoadBalancer) UpdateAddrs(addrs []string) {
	// ...
}

// Registers an 'orders' service node in 'us-east-1-b'.
func Example_registerOrdersServiceNode() {
	client, err := fuddle.Connect(
		context.TODO(),
		fuddle.Member{
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
		},
		// Seed addresses of Fuddle servers.
		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	)
	if err != nil {
		// handle err ...
	}
	defer client.Close()

	// ...
}

// Queries the set of active order service members in us-east-1 and subscribes
// to updates.
func Example_lookupOrderServiceNodes() {
	client, err := fuddle.Connect(
		context.TODO(),
		fuddle.Member{
			ID:      "frontend-040090b7",
			Service: "frontend",
			// ...
		},
		// Seed addresses of Fuddle servers.
		[]string{"192.168.1.1:8220", "192.168.1.2:8220", "192.168.1.3:8220"},
	)
	if err != nil {
		// handle err ...
	}
	defer client.Close()

	lb := &myLoadBalancer{}

	// ...

	// Subscribe fires whenever the registry changes, plus once immediately to
	// bootstrap.
	unsub := client.Subscribe(func() {
		var addrs []string
		for _, m := range client.Members() {
			// Filter the members to only include 'orders' service members in
			// us-east-1 which a status of 'active'.

			if m.Service != "orders" {
				continue
			}
			if !strings.HasPrefix(m.Locality, "aws.us-east-1") {
				continue
			}
			status, ok := m.Metadata["status"]
			if !ok || status != "active" {
				continue
			}
			ip, ok := m.Metadata["addr.rpc.ip"]
			if !ok {
				log.Println("[ERR] orders member missing addr.rpc.ip", m.ID)
				continue
			}
			port, ok := m.Metadata["addr.rpc.port"]
			if !ok {
				log.Println("[ERR] orders member missing addr.rpc.port", m.ID)
				continue
			}

			addrs = append(addrs, ip+":"+port)
		}

		if len(addrs) > 0 {
			lb.UpdateAddrs(addrs)
		} else {
			log.Println("[ERR] no active orders members found")
		}
	})
	defer unsub()

	// ...
}
