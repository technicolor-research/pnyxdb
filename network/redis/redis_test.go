/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package redis

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
)

const testKey = "teststream_network"

func TestBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := New(":6379", testKey, 0)
	require.Nil(t, err, "must connect to redis")

	_, _ = n.(*network).push.Do("DEL", testKey)

	fetched := make(chan proto.Message)
	go func() {
		for m := range n.Accept(ctx, func(proto.Message) bool { return true }) {
			fetched <- m
		}
	}()

	time.Sleep(20 * time.Millisecond)

	q := consensus.NewQuery()
	err = n.Broadcast(q)
	require.Nil(t, err, "must broadcast without error")

	q2 := consensus.NewQuery()
	_ = n.Broadcast(q2)

	require.Equal(t, q.Uuid, (<-fetched).(*consensus.Query).Uuid)
	require.Equal(t, q2.Uuid, (<-fetched).(*consensus.Query).Uuid)
}
