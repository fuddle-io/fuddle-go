package fuddle

import (
	rpc "github.com/fuddle-io/fuddle-rpc/go"
)

type Locality struct {
	Region           string
	AvailabilityZone string
}

type Member struct {
	ID       string
	Status   string
	Service  string
	Locality Locality
	Started  int64
	Revision string
	Metadata map[string]string
}

func (m *Member) toRPC() *rpc.MemberState {
	return &rpc.MemberState{
		Id:      m.ID,
		Status:  m.Status,
		Service: m.Service,
		Locality: &rpc.Locality{
			Region:           m.Locality.Region,
			AvailabilityZone: m.Locality.AvailabilityZone,
		},
		Started:  m.Started,
		Revision: m.Revision,
		Metadata: m.Metadata,
	}
}

func fromRPC(m *rpc.MemberState) Member {
	member := Member{
		ID:       m.Id,
		Service:  m.Service,
		Started:  m.Started,
		Revision: m.Revision,
		Metadata: m.Metadata,
	}
	if m.Locality != nil {
		member.Locality = Locality{
			Region:           m.Locality.Region,
			AvailabilityZone: m.Locality.AvailabilityZone,
		}
	}
	return member
}
