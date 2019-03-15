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

	"github.com/stretchr/testify/require"
)

func TestVersion_Matches(t *testing.T) {
	a := NewVersion([]byte("hello"))
	b := NewVersion([]byte("hello"))
	c := NewVersion([]byte("world"))

	require.Nil(t, a.Matches(b))
	require.Nil(t, b.Matches(a))

	require.Exactly(t, ErrVersionMismatch, a.Matches(c))
	require.Exactly(t, ErrVersionMismatch, c.Matches(a))

	require.NotNil(t, a.Matches(nil))
}

func TestVersion_Marshal(t *testing.T) {
	a := NewVersion([]byte("hello"))
	d, err := a.MarshalBinary()
	require.Nil(t, err)

	b := &Version{}
	err = b.UnmarshalBinary(d)
	require.Nil(t, err)
	require.Nil(t, a.Matches(b))
}
