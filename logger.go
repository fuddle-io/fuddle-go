package fuddle

import (
	"strings"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"go.uber.org/zap/zapcore"
)

type memberLogger struct {
	member *rpc.Member2
}

func newMemberLogger(m *rpc.Member2) memberLogger {
	return memberLogger{
		member: m,
	}
}

func (l memberLogger) MarshalLogObject(e zapcore.ObjectEncoder) error {
	e.AddString("state.id", l.member.State.Id)
	e.AddString("state.status", l.member.State.Status)
	e.AddString("state.service", l.member.State.Service)
	if l.member.State.Locality != nil {
		e.AddString("state.locality.region", l.member.State.Locality.Region)
		e.AddString("state.locality.az", l.member.State.Locality.AvailabilityZone)
	}
	e.AddString("state.started", l.member.State.Service)
	e.AddString("state.revision", l.member.State.Revision)

	e.AddString("liveness", strings.ToLower(l.member.Liveness.String()))

	e.AddString("version.owner", l.member.Version.OwnerId)
	e.AddInt64("version.timestamp", l.member.Version.Timestamp.Timestamp)
	e.AddUint64("version.counter", l.member.Version.Timestamp.Counter)

	e.AddInt64("expiry", l.member.Expiry)

	return nil
}
