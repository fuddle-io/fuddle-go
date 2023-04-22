package fuddle

import (
	"sync"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"go.uber.org/zap"
)

type subscriber struct {
	Callback func()
}

type registry struct {
	// members contains the members in the registry known by the client.
	members map[string]*rpc.Member2
	localID string

	subscribers map[*subscriber]interface{}

	// mu protects the above fields.
	mu sync.Mutex

	logger *zap.Logger
}

func newRegistry(member Member, logger *zap.Logger) *registry {
	members := make(map[string]*rpc.Member2)
	members[member.ID] = &rpc.Member2{
		State:    member.toRPC(),
		Liveness: rpc.Liveness_UP,
	}

	return &registry{
		members:     members,
		localID:     member.ID,
		subscribers: make(map[*subscriber]interface{}),
		logger:      logger,
	}
}

func (r *registry) LocalRPCMember() *rpc.MemberState {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.members[r.localID].State
}

func (r *registry) Members() []Member {
	r.mu.Lock()
	defer r.mu.Unlock()

	var members []Member
	for _, m := range r.members {
		members = append(members, fromRPC(m.State))
	}
	return members
}

func (r *registry) KnownVersions() map[string]*rpc.Version2 {
	r.mu.Lock()
	defer r.mu.Unlock()

	versions := make(map[string]*rpc.Version2)
	for id, m := range r.members {
		// Exclude the local member.
		if id == r.localID {
			continue
		}
		versions[id] = m.Version
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

func (r *registry) RemoteUpdate(m *rpc.Member2) {
	r.logger.Debug(
		"remote update",
		zap.Object("member", newMemberLogger(m)),
	)

	if m.State.Id == r.localID {
		return
	}

	if m.Liveness == rpc.Liveness_UP {
		r.updateMember(m)
	} else {
		r.removeMember(m.State.Id)
	}

	r.notifySubscribers()
}

func (r *registry) updateMember(m *rpc.Member2) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.members[m.State.Id] = m
}

func (r *registry) removeMember(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.members, id)
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
