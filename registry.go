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
	"sync"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
)

type subscriber struct {
	Callback func()
}

type registry struct {
	members map[string]*rpc.Member

	// localMembers is a set containing the members registered by this client.
	localMembers map[string]interface{}

	subscribers map[*subscriber]interface{}

	// mu protects the above fields.
	mu sync.Mutex
}

func newRegistry() *registry {
	return &registry{
		members:      make(map[string]*rpc.Member),
		localMembers: make(map[string]interface{}),
		subscribers:  make(map[*subscriber]interface{}),
	}
}

func (r *registry) Members(opts ...MembersOption) []Member {
	r.mu.Lock()
	defer r.mu.Unlock()

	options := &membersOptions{}
	for _, o := range opts {
		o.apply(options)
	}

	var members []Member
	for _, m := range r.members {
		m := fromRPC(m)
		if options.filter == nil || options.filter.Match(m) {
			members = append(members, m)
		}
	}
	return members
}

func (r *registry) LocalMemberIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var memberIDs []string
	for id := range r.localMembers {
		memberIDs = append(memberIDs, id)
	}
	return memberIDs
}

func (r *registry) LocalMembers() []Member {
	r.mu.Lock()
	defer r.mu.Unlock()

	var members []Member
	for id := range r.localMembers {
		members = append(members, fromRPC(r.members[id]))
	}
	return members
}

func (r *registry) KnownVersions() map[string]uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	versions := make(map[string]uint64)
	for id, m := range r.members {
		// Exclude local members from the known versions as the server doesn't
		// send us our own members.
		if _, ok := r.localMembers[id]; !ok {
			versions[id] = m.Version
		}
	}
	return versions
}

func (r *registry) Subscribe(cb func()) func() {
	r.mu.Lock()

	sub := &subscriber{
		Callback: cb,
	}
	r.subscribers[sub] = struct{}{}

	r.mu.Unlock()

	// Ensure calling outside of the mutex.
	cb()

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		delete(r.subscribers, sub)
	}
}

func (r *registry) RegisterLocal(member *rpc.Member) {
	r.mu.Lock()

	r.members[member.Id] = member
	r.localMembers[member.Id] = struct{}{}

	r.mu.Unlock()

	r.notifySubscribers()
}

func (r *registry) UnregisterLocal(id string) {
	r.mu.Lock()

	delete(r.members, id)
	delete(r.localMembers, id)

	r.mu.Unlock()

	r.notifySubscribers()
}

func (r *registry) UpdateMetadataLocal(id string, metadata map[string]string) {
	r.mu.Lock()

	member, ok := r.members[id]
	if !ok {
		r.mu.Unlock()
		return
	}

	for k, v := range metadata {
		member.Metadata[k] = v
	}

	r.mu.Unlock()

	r.notifySubscribers()
}

func (r *registry) ApplyRemoteUpdate(update *rpc.MemberUpdate) {
	r.mu.Lock()

	// Ignore updates about local members.
	if _, ok := r.localMembers[update.Id]; ok {
		r.mu.Unlock()
		return
	}

	switch update.UpdateType {
	case rpc.MemberUpdateType_REGISTER:
		r.applyRegisterUpdateLocked(update)
	case rpc.MemberUpdateType_UNREGISTER:
		r.applyUnregisterUpdateLocked(update)
	case rpc.MemberUpdateType_STATE:
		r.applyStateUpdateLocked(update)
	}

	r.mu.Unlock()

	r.notifySubscribers()
}

func (r *registry) applyRegisterUpdateLocked(update *rpc.MemberUpdate) {
	r.members[update.Id] = update.Member
}

func (r *registry) applyUnregisterUpdateLocked(update *rpc.MemberUpdate) {
	delete(r.members, update.Id)
}

func (r *registry) applyStateUpdateLocked(update *rpc.MemberUpdate) {
	r.members[update.Id] = update.Member
}

func (r *registry) notifySubscribers() {
	r.mu.Lock()

	// Copy the subscribers to avoid calling with the mutex locked.
	subscribers := make([]*subscriber, 0, len(r.subscribers))
	for sub := range r.subscribers {
		subscribers = append(subscribers, sub)
	}

	r.mu.Unlock()

	for _, sub := range subscribers {
		sub.Callback()
	}
}
