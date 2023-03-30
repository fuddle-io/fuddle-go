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
	"github.com/fuddle-io/fuddle-go/internal/wildcard"
)

// Filter specifies a member filter.
//
// This maps a service name (which may include wildcards with '*') to a service
// filter.
//
// Any members whose service don't match any of those listed are discarded.
type Filter map[string]ServiceFilter

func (f *Filter) Match(member Member) bool {
	// Must match at least one service, and all service filters where there
	// is a service name match.
	match := false
	for filterService, filter := range *f {
		if wildcard.Match(filterService, member.Service) {
			match = true

			if !filter.Match(member) {
				return false
			}
		}
	}
	return match
}

// ServiceFilter specifies a member filter that applies to all members in a service.
type ServiceFilter struct {
	// Locality is a list of localities (which may include wildcards with '*'),
	// where the members locality must match at least on of the listed localities.
	Locality []string

	// Metadata contains the state filter.
	Metadata MetadataFilter
}

func (f *ServiceFilter) Match(member Member) bool {
	// If there are no localites allow all.
	if f.Locality != nil {
		// The member locality must match at least one filter locality.
		match := false
		for _, filterLoc := range f.Locality {
			if wildcard.Match(filterLoc, member.Locality) {
				match = true
			}
		}
		if !match {
			return false
		}
	}

	return f.Metadata.Match(member)
}

// MetadataFilter specifies a member filter that discards members whose metadata
// doesn't match the state listed.
//
// To match, for each filter key, the member must include a value for that key
// and match at least on of the filters for that key.
//
// The filter values may include wildcards, though the keys cannot.
type MetadataFilter map[string][]string

func (f *MetadataFilter) Match(member Member) bool {
	for filterKey, filterValues := range *f {
		v, ok := member.Metadata[filterKey]
		// If the filter key is not in the member, its not a match.
		if !ok {
			return false
		}

		// The value must match at least one filter value.
		match := false
		for _, filterValue := range filterValues {
			if wildcard.Match(filterValue, v) {
				match = true
			}
		}
		if !match {
			return false
		}
	}

	return true
}
