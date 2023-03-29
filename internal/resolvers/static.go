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

package resolvers

import (
	"google.golang.org/grpc/resolver"
)

type StaticResolverBuilder struct {
	addrs []string
}

func NewStaticResolverBuilder(addrs []string) *StaticResolverBuilder {
	return &StaticResolverBuilder{
		addrs: addrs,
	}
}

func (s *StaticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	var addrs []resolver.Address
	for _, addr := range s.addrs {
		addrs = append(addrs, resolver.Address{Addr: addr})
	}

	r := &StaticResolver{
		target: target,
		cc:     cc,
		addrs:  addrs,
	}
	r.start()
	return r, nil
}

func (s *StaticResolverBuilder) Scheme() string {
	return "static"
}

type StaticResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
	addrs  []resolver.Address
}

func (s *StaticResolver) start() {
	s.updateAddresses(s.addrs)
}

func (s *StaticResolver) ResolveNow(resolver.ResolveNowOptions) {
	s.updateAddresses(s.addrs)
}

func (s *StaticResolver) Close() {
}

func (s *StaticResolver) updateAddresses(addrs []resolver.Address) {
	//nolint
	s.cc.UpdateState(resolver.State{Addresses: addrs})
}

var _ resolver.Builder = &StaticResolverBuilder{}
