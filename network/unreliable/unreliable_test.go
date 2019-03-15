/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package unreliable

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnreliableLatency(t *testing.T) {
	n := New(nil, Parameters{
		Seed:          1234,
		MinLatency:    10 * time.Millisecond,
		MedianLatency: 100 * time.Millisecond,
		MaxLatency:    1 * time.Second,
	}).(*network)

	var down, up int
	var max time.Duration
	for i := 0; i < 10000; i++ {
		d := n.randLatency()
		require.True(t, d >= n.MinLatency, "minimum latency should be respected")
		require.True(t, d <= n.MaxLatency, "maximum latency should be respected")

		if d > max {
			max = d
		}

		if d < n.MedianLatency {
			down++
		} else {
			up++
		}
	}

	require.InEpsilon(t, down, up, 0.01, "median should be respected with lower than 1% error")
}
