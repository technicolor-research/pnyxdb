/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package bbc

import (
	"context"
	fmt "fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/network/redis"
	"github.com/technicolor-research/pnyxdb/tests"
)

func runVetoEngine(t *testing.T, choices []bool, proof *consensus.Proof, expected bool) {
	n, err := redis.New(":6379", "teststream_veto", 0)
	require.Nil(t, err, "should establish connection to redis")

	id := strconv.Itoa(int(time.Now().UnixNano()))
	ctx := context.Background()

	var wg sync.WaitGroup
	keyrings := tests.GetTestKeyRings(t, len(choices))

	for i := 0; i < len(choices); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ve, err := NewVetoEngine(n, keyrings[i], len(choices))
			require.Nil(t, err, "should create a correct veto engine")

			var proofs []*consensus.Proof
			if !choices[i] {
				proofs = append(proofs, proof)
			}

			decision, dp, err := ve.Execute(ctx, id, choices[i], proofs)
			require.Nil(t, err, "execute should not result in an error")
			require.Equal(t, expected, decision, fmt.Sprintf("decision %d is invalid", i))

			if proof == nil {
				require.Equal(t, 0, len(dp), fmt.Sprintf("decision proof %d should not exist", i))
			} else {
				require.Equal(t, 1, len(dp), fmt.Sprintf("decision proof %d should exist", i))
				require.Equal(t, proof.GetQuery().Uuid, dp[0].GetQuery().Uuid, fmt.Sprintf("decision proof %d is invalid", i))
			}

		}(i)
	}

	wg.Wait()
}

func TestVetoEngine(t *testing.T) {
	choices := make([]bool, 100)
	proof := &consensus.Proof{
		Content: &consensus.Proof_Query{
			Query: consensus.NewQuery(),
		},
	}

	t.Run("CompleteDisagreement", func(t *testing.T) {
		runVetoEngine(t, choices, proof, false)
	})

	t.Run("CompleteAgreement", func(t *testing.T) {
		for i := range choices {
			choices[i] = true
		}
		runVetoEngine(t, choices, nil, true)
	})

	t.Run("OneVeto", func(t *testing.T) {
		choices[42] = false
		runVetoEngine(t, choices, proof, false)
	})
}
