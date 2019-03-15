/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngine_Dump(t *testing.T) {
	qs := newQueryStore()
	qs.threshold = 2
	e := &Engine{qs: qs}

	buffer := &bytes.Buffer{}
	err := e.Dump(buffer)
	require.Nil(t, err, "should be able to dump empty query store")

	q := NewQuery()
	eq1 := &Endorsement{Emitter: "1", Uuid: q.Uuid}
	eq2 := &Endorsement{Emitter: "2", Uuid: q.Uuid}

	qs.AddQuery(q)
	qs.AddEndorsement(eq1)

	buffer.Reset()
	err = e.Dump(buffer)
	require.Nil(t, err, "should be able to dump simple query store")

	qs.AddEndorsement(eq2)
	require.True(t, qs.isApplicable(q.Uuid))

	buffer.Reset()
	err = e.Dump(buffer)
	require.Nil(t, err, "should be able to dump query store with applicable queries")

	qs2 := newQueryStore()
	qs2.threshold = 2
	e2 := &Engine{qs: qs2}
	err = e2.Load(buffer)
	require.Nil(t, err, "should be able to load query store from buffer")

	require.True(t, qs2.isApplicable(q.Uuid))
}
