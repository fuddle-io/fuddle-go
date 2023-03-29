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
	"time"

	"go.uber.org/zap"
)

type options struct {
	connectAttemptTimeout time.Duration
	logger                *zap.Logger
	grpcLoggerVerbosity   int
}

func defaultOptions() *options {
	return &options{
		connectAttemptTimeout: time.Second * 4,
		logger:                zap.NewNop(),
		grpcLoggerVerbosity:   0,
	}
}

type Option interface {
	apply(*options)
}

type connectAttemptTimeoutOption struct {
	timeout time.Duration
}

func (o connectAttemptTimeoutOption) apply(opts *options) {
	opts.connectAttemptTimeout = o.timeout
}

// WithConnectAttemptTimeout is the timeout for each connect attempt.
// This is different from the overall connect timeout which may attempt multiple
// addresses.
//
// Defaults to 4 seconds.
func WithConnectAttemptTimeout(timeout time.Duration) Option {
	return connectAttemptTimeoutOption{timeout: timeout}
}

type loggerOption struct {
	logger *zap.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.logger = o.logger
}

func WithLogger(logger *zap.Logger) Option {
	return loggerOption{logger: logger}
}

type grpcLoggerVerbosityOption struct {
	v int
}

func (o grpcLoggerVerbosityOption) apply(opts *options) {
	opts.grpcLoggerVerbosity = o.v
}

// WithGRPCLoggerVerbosity adds gRPC logging to stdout and stderr. Note this is
// independent from WithLogger and should only be used for debugging rather
// than production code.
//
// Defaults to 0 to disable gRPC logs.
func WithGRPCLoggerVerbosity(v int) Option {
	return grpcLoggerVerbosityOption{v: v}
}
