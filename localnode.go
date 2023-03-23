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
	"context"
	"fmt"

	rpc "github.com/fuddle-io/fuddle-rpc/go"
	"go.uber.org/zap"
)

// LocalNode manages a local nodes entry into the registry.
type LocalNode struct {
	id     string
	client rpc.RegistryClient
	logger *zap.Logger
}

func newLocalNode(id string, client rpc.RegistryClient, logger *zap.Logger) *LocalNode {
	return &LocalNode{
		id:     id,
		client: client,
		logger: logger,
	}
}

// UpdateMetadata updates the state of this node, which will update the nodes
// state in the registry.
func (n *LocalNode) UpdateMetadata(ctx context.Context, update map[string]string) error {
	req := &rpc.UpdateNodeRequest{
		NodeId:   n.id,
		Metadata: update,
	}

	resp, err := n.client.UpdateNode(ctx, req)
	if err != nil {
		n.logger.Error(
			"failed to update node",
			zap.String("id", n.id),
			zap.Error(err),
		)

		return fmt.Errorf("fuddle: update metadata: %w", err)
	}
	if resp.Error != nil {
		n.logger.Error(
			"failed to update node",
			zap.String("id", n.id),
			zap.String("error", resp.Error.Description),
		)

		return fmt.Errorf("fuddle: update metadata: %s", resp.Error.Description)
	}

	n.logger.Debug("node metadata updated", zap.String("id", n.id))

	return nil
}

// Unregister removes this node from the registry.
func (n *LocalNode) Unregister(ctx context.Context) error {
	req := &rpc.UnregisterRequest{
		NodeId: n.id,
	}
	resp, err := n.client.Unregister(ctx, req)
	if err != nil {
		n.logger.Error(
			"failed to unregister node",
			zap.String("id", n.id),
			zap.Error(err),
		)

		return fmt.Errorf("fuddle: unregister: %w", err)
	}
	if resp.Error != nil {
		n.logger.Error(
			"failed to unregister node",
			zap.String("id", n.id),
			zap.String("error", resp.Error.Description),
		)

		return fmt.Errorf("fuddle: unregister: %s", resp.Error.Description)
	}

	n.logger.Debug("node unregistered", zap.String("id", n.id))

	return nil
}
