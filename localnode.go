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

// LocalNode manages a local nodes entry into the registry.
type LocalNode struct {
}

// UpdateMetadata updates the state of this node, which will update the nodes
// state in the registry.
func (n *LocalNode) UpdateMetadata(update map[string]string) error {
	return nil
}

// Unregister removes this node from the registry.
func (n *LocalNode) Unregister() error {
	return nil
}
