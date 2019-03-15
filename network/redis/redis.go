/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package redis implements a mockup, test-only, broadcast network.
package redis

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/network/protocol"
)

type network struct {
	push redis.Conn
	pool *redis.Pool

	sync.Mutex

	streamName string
}

// New returns a new redis-based centralized network.
// It should only be used for demonstration purposes.
func New(address, streamName string, database int) (n consensus.Network, err error) {
	pool := &redis.Pool{
		MaxIdle:     64,
		IdleTimeout: 2 * time.Minute,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", address, redis.DialDatabase(database)) },
	}

	push, err := pool.Dial()
	if err != nil {
		return
	}

	n = &network{
		push:       push,
		pool:       pool,
		streamName: streamName,
	}
	return
}

func (n *network) Broadcast(m proto.Message) error {
	data, err := protocol.Pack(m)
	if err != nil {
		return err
	}

	n.Lock()
	defer n.Unlock()

	_, err = n.push.Do("XADD", n.streamName, "MAXLEN", "~", "1024", "*", "raw", data)
	return err
}

func (n *network) Accept(ctx context.Context, acceptor consensus.MessageAcceptor) <-chan proto.Message {
	output := make(chan proto.Message)

	go func() {
		lastSeen := "0"
		pull := n.pool.Get()
		defer func() { _ = pull.Close() }()
		defer close(output)

		for {
			streams, err := redis.Values(pull.Do("XREAD", "COUNT", "20", "BLOCK", "10000", "STREAMS", n.streamName, lastSeen))
			if err != nil || len(streams) == 0 {
				time.Sleep(time.Second) // Add some cooldown
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}

			events := streams[0].([]interface{})[1].([]interface{})
			for _, event := range events {
				eventData := event.([]interface{})
				lastSeen = eventData[0].(string)
				data := eventData[1].([]interface{})[1].([]byte)
				m, err := protocol.Unpack(bytes.NewBuffer(data))
				if err != nil || !acceptor(m) {
					continue
				}

				select {
				case output <- m:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return output
}

func (n *network) Close() error {
	return n.push.Close()
}
