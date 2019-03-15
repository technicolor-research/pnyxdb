/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package unreliable can arbitraly make a network unreliable for test purpose.
package unreliable

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/technicolor-research/pnyxdb/consensus"
)

var log2 = math.Log(2)

// Parameters hold unreliable network characterics
type Parameters struct {
	Seed int64

	MinLatency    time.Duration
	MedianLatency time.Duration
	MaxLatency    time.Duration
}

// New returns a new simulated unreliable network from a parent network
func New(parent consensus.Network, p Parameters) consensus.Network {
	if p.Seed <= 0 {
		p.Seed = time.Now().UnixNano()
	}

	return &network{
		Network:    parent,
		Parameters: p,
		rng:        rand.New(rand.NewSource(p.Seed)),
	}
}

type network struct {
	consensus.Network
	Parameters
	sync.Mutex // rng is not thread-safe

	rng *rand.Rand
}

func (n *network) Broadcast(m proto.Message) error {
	go func() {
		time.Sleep(n.randLatency())
		_ = n.Network.Broadcast(m)
	}()

	return nil
}

func (n *network) Accept(ctx context.Context, acceptor consensus.MessageAcceptor) <-chan proto.Message {
	output := make(chan proto.Message)
	parentOutput := n.Network.Accept(ctx, acceptor)

	go func() {
		defer close(output)
		for m := range parentOutput {
			d := n.randLatency()

			go func(m proto.Message) {
				select {
				case <-ctx.Done():
				case <-time.After(d):
					output <- m
				}
			}(m)
		}
	}()

	return output
}

func (n *network) randExp(median float64) float64 {
	n.Lock()
	defer n.Unlock()

	factor := median / log2
	return rand.ExpFloat64() * factor
}

func (n *network) randLatency() time.Duration {
	d := time.Duration(n.randExp(float64(n.MedianLatency)))

	if d < n.MinLatency {
		return n.MinLatency
	}

	if d > n.MaxLatency {
		return n.MaxLatency
	}

	return d
}
