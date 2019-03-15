/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package encoding contains database types and (un)marshalling methods.
package encoding

import "encoding/binary"

var defaultEncoding = binary.LittleEndian

func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	defaultEncoding.PutUint64(buf, u)
	return buf
}

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return defaultEncoding.Uint64(b)
}
