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
	"context"
)

// Fuddle is a client for the Fuddle registry which can be used to subscribe to
// registry updates and register members.
type Fuddle struct{}

// Connect connects to the Fuddle registry and starts streaming registry
// updates.
//
// The seed addresses are addresses of Fuddle nodes in the cluster.
//
// Returns an error if the client fails to connect to a Fuddle node before the
// given context is cancelled.
func Connect(ctx context.Context, seeds []string, opts ...Option) (*Fuddle, error) {
	return &Fuddle{}, nil
}
