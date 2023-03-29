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

package fuddle

import (
	rpc "github.com/fuddle-io/fuddle-rpc/go"
)

type Member struct {
	ID       string
	Service  string
	Locality string
	Created  int64
	Revision string
	Metadata map[string]string
}

func fromRPC(m *rpc.Member) Member {
	return Member{
		ID:       m.Id,
		Service:  m.Service,
		Locality: m.Locality,
		Created:  m.Created,
		Revision: m.Revision,
		Metadata: m.Metadata,
	}
}

func (m *Member) toRPC(clientID string) *rpc.Member {
	return &rpc.Member{
		Id:       m.ID,
		ClientId: clientID,
		Service:  m.Service,
		Locality: m.Locality,
		Created:  m.Created,
		Revision: m.Revision,
		Metadata: m.Metadata,
	}
}
