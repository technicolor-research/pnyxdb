/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"context"
	"io"
	"sync"

	proto "github.com/golang/protobuf/proto"
)

// Store is the interface storage drivers must implement.
type Store interface {
	sync.Locker
	io.Closer

	// Get returns the value and the version stored currently for the specified key.
	Get(key string) (value []byte, version *Version, err error)
	// Set sets the value and the version that must be stored for the specified key.
	Set(key string, value []byte, version *Version) error
	// SetBatch executes the given "Set" operations in a atomic way.
	SetBatch(keys []string, values [][]byte, versions []*Version) error
	// List returns the map of keys with their values.
	List() (map[string]*Version, error)
}

// Network is the interface network adapters must implement.
type Network interface {
	io.Closer

	Broadcast(m proto.Message) error
	Accept(ctx context.Context, acceptor MessageAcceptor) <-chan proto.Message
}

// RecoveryManager is a interface that can optionally be proposed by Networks for
// key recovery support (after a crash or network partition).
type RecoveryManager interface {
	RequestRecovery(ctx context.Context, key string) (*RecoveryResponse, error)
	AcceptRecovery(ctx context.Context, handler RecoveryHandler)
}

// RecoveryHandler is a callback used by the RecoveryManager.
type RecoveryHandler func(*RecoveryRequest) (*RecoveryResponse, error)

// MessageAcceptor is a filter that can be used to filter incoming proto messages.
type MessageAcceptor func(proto.Message) bool

// BBCEngine is the interface for binary Byzantine consensus engine.
type BBCEngine interface {
	Execute(context.Context, string, bool, []*Proof) (bool, []*Proof, error)
}
