/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/consensus/bbc"
	"github.com/technicolor-research/pnyxdb/network/redis"
	"github.com/technicolor-research/pnyxdb/network/unreliable"
	"github.com/technicolor-research/pnyxdb/storage/boltdb"
)

func TestEngine(t *testing.T) {
	n := 20
	w := 20
	s := strconv.Itoa(int(time.Now().UnixNano()))

	p := unreliable.Parameters{
		MinLatency:    1 * time.Millisecond,
		MedianLatency: 30 * time.Millisecond,
		MaxLatency:    200 * time.Millisecond,
	}

	keyrings := GetTestKeyRings(t, n)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(chan []byte, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			testdir, err := ioutil.TempDir("", "consensus_engine_")
			require.Nil(t, err)
			defer func() { _ = os.RemoveAll(testdir) }()

			store, err := boltdb.New(filepath.Join(testdir, "db"))
			require.Nil(t, err)
			defer store.Close()

			network, err := redis.New(":6379", "stream_"+s, 0)
			require.Nil(t, err)
			defer network.Close()

			unreliableNetwork := unreliable.New(network, p)

			ve, err := bbc.NewVetoEngine(unreliableNetwork, keyrings[i], n)
			require.Nil(t, err)

			engine := consensus.NewEngine(store, unreliableNetwork, ve, keyrings[i], w)
			err = engine.Run(ctx)
			require.Nil(t, err, "should run without error")

			if i < 3 {
				q := consensus.NewQuery()
				q.SetTimeout(time.Duration(i) * time.Second)
				fmt.Println("Query", i, "is", q.Uuid)
				q.Operations = []*consensus.Operation{
					{Key: "a", Op: consensus.Operation_CONCAT, Data: []byte{byte(i)}},
				}
				err = engine.Submit(q)
				require.Nil(t, err, "should submit new query without error")
			}

			<-ctx.Done()

			value, _, _ := store.Get("a")
			out <- value
		}(i)
	}

	var ref []byte
	for i := 0; i < n; i++ {
		state := <-out
		if i == 0 {
			ref = state
		}

		require.Equal(t, ref, state, "states must be consistent")
	}

	fmt.Println(ref)
	wg.Wait()
}
