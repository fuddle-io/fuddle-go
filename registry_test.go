package fuddle

import (
	"math/rand"
	"testing"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRegistry_RemoteUpdateAddMember(t *testing.T) {
	localMember := randomMember("local")
	reg := newRegistry(fromRPC(localMember), zap.NewNop())

	addedMember := randomMember("member-1")
	reg.RemoteUpdate(&rpc.Member2{
		State:    addedMember,
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})

	assert.Equal(t, []Member{fromRPC(localMember), fromRPC(addedMember)}, reg.Members())
}

func TestRegistry_RemoteIgnoreLocalMember(t *testing.T) {
	localMember := randomMember("local")
	reg := newRegistry(fromRPC(localMember), zap.NewNop())

	reg.RemoteUpdate(&rpc.Member2{
		State:    randomMember("local"),
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})

	// Local member should be unchanged.
	assert.Equal(t, []Member{fromRPC(localMember)}, reg.Members())
}

func TestRegistry_RemoteUpdateRemoveMember(t *testing.T) {
	localMember := randomMember("local")
	reg := newRegistry(fromRPC(localMember), zap.NewNop())

	reg.RemoteUpdate(&rpc.Member2{
		State:    randomMember("member-1"),
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})
	reg.RemoteUpdate(&rpc.Member2{
		State: &rpc.MemberState{
			Id: "member-1",
		},
		Liveness: rpc.Liveness_LEFT,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})

	assert.Equal(t, []Member{fromRPC(localMember)}, reg.Members())
}

func TestRegistry_KnownVersions(t *testing.T) {
	localMember := randomMember("local")
	reg := newRegistry(fromRPC(localMember), zap.NewNop())

	reg.RemoteUpdate(&rpc.Member2{
		State:    randomMember("member-1"),
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})
	reg.RemoteUpdate(&rpc.Member2{
		State:    randomMember("member-2"),
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-2",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 456,
			},
		},
	})

	assert.Equal(t, map[string]*rpc.Version2{
		"member-1": &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
		"member-2": &rpc.Version2{
			OwnerId: "remote-2",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 456,
			},
		},
	}, reg.KnownVersions())
}

func TestRegistry_Subscribe(t *testing.T) {
	localMember := randomMember("local")
	reg := newRegistry(fromRPC(localMember), zap.NewNop())

	count := 0
	reg.Subscribe(func() {
		count++
	})

	reg.RemoteUpdate(&rpc.Member2{
		State:    randomMember("member-1"),
		Liveness: rpc.Liveness_UP,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})
	reg.RemoteUpdate(&rpc.Member2{
		State: &rpc.MemberState{
			Id: "member-1",
		},
		Liveness: rpc.Liveness_LEFT,
		Version: &rpc.Version2{
			OwnerId: "remote-1",
			Timestamp: &rpc.MonotonicTimestamp{
				Timestamp: 123,
			},
		},
	})

	assert.Equal(t, 3, count)
}

func randomMember(id string) *rpc.MemberState {
	if id == "" {
		id = uuid.New().String()
	}
	return &rpc.MemberState{
		Id:      id,
		Service: uuid.New().String(),
		Locality: &rpc.Locality{
			Region:           uuid.New().String(),
			AvailabilityZone: uuid.New().String(),
		},
		Started:  rand.Int63(),
		Revision: uuid.New().String(),
		Metadata: map[string]string{
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
			uuid.New().String(): uuid.New().String(),
		},
	}
}
