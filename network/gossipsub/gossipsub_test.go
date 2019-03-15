/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package gossipsub

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
)

func TestGossipSub(t *testing.T) {
	h, _ := libp2p.New(context.Background())
	p := Defaults(h)
	p.BootstrapAddrs = []string{}

	n, err := New(p)
	require.Nil(t, err)

	time.Sleep(20 * time.Millisecond)

	q := consensus.NewQuery()
	err = n.Broadcast(q)
	require.Nil(t, err, "must broadcast without error")

	q2 := consensus.NewQuery()
	_ = n.Broadcast(q2)
	fetched := make(chan proto.Message)

	go func() {
		time.Sleep(time.Second)
		for m := range n.Accept(p.Ctx, func(proto.Message) bool { return true }) {
			fetched <- m
		}
	}()

	require.Equal(t, q.Uuid, (<-fetched).(*consensus.Query).Uuid)
	require.Equal(t, q2.Uuid, (<-fetched).(*consensus.Query).Uuid)
}
