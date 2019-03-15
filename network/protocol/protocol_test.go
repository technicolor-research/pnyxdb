/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
)

func Test_Call_Pack(t *testing.T) {
	data, err := Pack(consensus.NewQuery())
	require.Nil(t, err)
	require.Exactly(t, byte(0x01), data[0])

	l, n := binary.Uvarint(data[1:])
	require.Exactly(t, l, uint64(len(data[1+n:])))
}

func Test_Call_Unpack(t *testing.T) {
	q := consensus.NewQuery()
	data, _ := Pack(q)
	buffer := bytes.NewBuffer(data)

	q2, err := Unpack(buffer)
	require.Nil(t, err)
	require.IsType(t, q, q2, "must retrieve the same type")
	require.Exactly(t, q.Uuid, q2.(*consensus.Query).Uuid)
}

func Test_Call_Unpack_Invalid(t *testing.T) {
	check := func(data []byte, msg string) {
		_, err := Unpack(bytes.NewBuffer(data))
		require.NotNil(t, err, msg)
	}

	check([]byte{}, "must handle empty data")
	check([]byte{0xf2}, "must handle invalid function")
	check([]byte{0x01, 0xff}, "must handle invalid uvarint")
	check([]byte{0x01, 0xff}, "must handle invalid uvarint")
	check([]byte{0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}, "must handle too large uvarint")
	check([]byte{0x01, 0x02, 0xff}, "must handle too small raw protobuf")
}
