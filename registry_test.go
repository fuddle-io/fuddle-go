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

func TestRegistry_RegisterLocal(t *testing.T) {
	r := newRegistry()

	member := randomMember()
	r.RegisterLocal(member.toRPC("my-client"))

	assert.Equal(t, []Member{member}, r.Members())
}

func TestRegistry_UnregisterLocal(t *testing.T) {
	r := newRegistry()

	member := randomMember()
	r.RegisterLocal(member.toRPC("my-client"))
	r.UnregisterLocal(member.ID)

	assert.Equal(t, []Member(nil), r.Members())
}

func TestRegistry_UpdateMetadataLocal(t *testing.T) {
	r := newRegistry()

	member := randomMember()
	r.RegisterLocal(member.toRPC("my-client"))

	update := randomMetadata()
	r.UpdateMetadataLocal(member.ID, update)

	expectedMember := member
	for k, v := range update {
		expectedMember.Metadata[k] = v
	}

	assert.Equal(t, []Member{expectedMember}, r.Members())
}

func TestRegistry_ApplyRemoteRegister(t *testing.T) {
	r := newRegistry()

	var registered []Member
	for i := 0; i != 5; i++ {
		member := randomMember()
		r.ApplyRemoteUpdate(memberToRegisterUpdate(member))
		registered = append(registered, member)
	}

	sort.Slice(registered, func(i, j int) bool {
		return registered[i].ID < registered[j].ID
	})

	members := r.Members()
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID < members[j].ID
	})

	assert.Equal(t, registered, members)
}

func TestRegistry_ApplyRemoteUnregister(t *testing.T) {
	r := newRegistry()

	var registered []Member
	for i := 0; i != 5; i++ {
		member := randomMember()
		r.ApplyRemoteUpdate(memberToRegisterUpdate(member))
		registered = append(registered, member)
	}

	for _, m := range registered {
		r.ApplyRemoteUpdate(memberToUnregisterUpdate(m))
	}

	assert.Equal(t, 0, len(r.Members()))
}

func TestRegistry_ApplyRemoteStateUpdate(t *testing.T) {
	r := newRegistry()

	var registered []Member
	for i := 0; i != 5; i++ {
		member := randomMember()
		r.ApplyRemoteUpdate(memberToRegisterUpdate(member))
		registered = append(registered, member)
	}

	var updated []Member
	for _, m := range registered {
		metadata := randomMetadata()
		for k, v := range metadata {
			m.Metadata[k] = v
		}

		r.ApplyRemoteUpdate(memberToStateUpdate(m))
		updated = append(updated, m)
	}

	sort.Slice(updated, func(i, j int) bool {
		return updated[i].ID < updated[j].ID
	})

	members := r.Members()
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID < members[j].ID
	})

	assert.Equal(t, updated, members)
}

func TestRegistry_ApplyRemoteUpdateIgnoresLocalMembers(t *testing.T) {
	r := newRegistry()

	member := randomMember()
	r.RegisterLocal(member.toRPC("my-client"))

	// Unregistering the local member from a remote updates should be ignored.
	r.ApplyRemoteUpdate(memberToUnregisterUpdate(member))

	assert.Equal(t, []Member{member}, r.Members())
}

func TestRegistry_Subscribe(t *testing.T) {
	r := newRegistry()

	count := 0
	unsubscribe := r.Subscribe(func() {
		count++
	})

	m := randomMember()
	r.ApplyRemoteUpdate(memberToRegisterUpdate(m))
	assert.Equal(t, 2, count)

	r.ApplyRemoteUpdate(memberToStateUpdate(m))
	assert.Equal(t, 3, count)

	r.ApplyRemoteUpdate(memberToUnregisterUpdate(m))
	assert.Equal(t, 4, count)

	// Unsubscribe and should not get any more updates.
	unsubscribe()

	r.ApplyRemoteUpdate(memberToRegisterUpdate(randomMember()))
	assert.Equal(t, 4, count)
}

func randomMember() Member {
	return Member{
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

func memberToRegisterUpdate(m Member) *rpc.MemberUpdate {
	return &rpc.MemberUpdate{
		Id:         m.ID,
		UpdateType: rpc.MemberUpdateType_REGISTER,
		Member:     m.toRPC("my-client"),
	}
}

func memberToUnregisterUpdate(m Member) *rpc.MemberUpdate {
	return &rpc.MemberUpdate{
		Id:         m.ID,
		UpdateType: rpc.MemberUpdateType_UNREGISTER,
	}
}

func memberToStateUpdate(m Member) *rpc.MemberUpdate {
	return &rpc.MemberUpdate{
		Id:         m.ID,
		UpdateType: rpc.MemberUpdateType_STATE,
		Member:     m.toRPC("my-client"),
	}
}
