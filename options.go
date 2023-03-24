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
	"time"

	"go.uber.org/zap"
)

type options struct {
	logger            *zap.Logger
	connectTimeout    time.Duration
	heartbeatInterval time.Duration
	filter            *Filter
}

type Option interface {
	apply(*options)
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

type connectTimeoutOption struct {
	timeout time.Duration
}

func (o connectTimeoutOption) apply(opts *options) {
	opts.connectTimeout = o.timeout
}

// WithConnectTimeout defines the time to wait for each connection attempt
// before timing out. Default to 1 second.
func WithConnectTimeout(timeout time.Duration) Option {
	return connectTimeoutOption{timeout: timeout}
}

type heartbeatIntervalOption struct {
	interval time.Duration
}

func (o heartbeatIntervalOption) apply(opts *options) {
	opts.heartbeatInterval = o.interval
}

func WithHeartbeatInterval(interval time.Duration) Option {
	return heartbeatIntervalOption{interval: interval}
}

type filterOption struct {
	filter *Filter
}

func (o filterOption) apply(opts *options) {
	opts.filter = o.filter
}

// WithFilter filters the returned set of nodes.
func WithFilter(f Filter) Option {
	return filterOption{filter: &f}
}
