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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	clusterAddr = "127.0.0.1:8220"
)

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
