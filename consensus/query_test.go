/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueryTimeout(t *testing.T) {
	d := 50 * time.Millisecond
	q := NewQuery()
	q.SetTimeout(d)
	require.False(t, q.Expired())
	require.False(t, q.ExpiredSince(d))

	time.Sleep(d)

	require.True(t, q.Expired())
	require.False(t, q.ExpiredSince(d))

	time.Sleep(d)

	require.True(t, q.Expired())
	require.True(t, q.ExpiredSince(d))
}
