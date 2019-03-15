/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package protocol holds the PnyxDB peer-to-peer protocol.
//
// Paquet format:
// - 1 byte for type identifier selection
// - n bytes for data length specification (uvarint)
// - remaining bytes containing data
package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"reflect"

	"github.com/golang/protobuf/proto"
)

var typeIdentifiers = []string{
	"",
	"consensus.Query",
	"consensus.Endorsement",
	"consensus.StartCheckpoint",
	"reserved",
	"reserved",
	"reserved",
	"consensus.RecoveryRequest",
	"consensus.RecoveryResponse",
	"reserved",
	"bbc.Choice",
}

func getTypeFromName(name string) byte {
	for i, n := range typeIdentifiers {
		if n == name {
			return byte(i)
		}
	}

	return 0
}

// Pack packs
func Pack(m proto.Message) (data []byte, err error) {
	// Generate protobuf wire data
	raw, err := proto.Marshal(m)
	if err != nil {
		return
	}

	// Make a arbitrary size data buffer
	data = make([]byte, 1+binary.MaxVarintLen64)
	data[0] = getTypeFromName(proto.MessageName(m))
	n := binary.PutUvarint(data[1:], uint64(len(raw)))

	// Add raw data
	data = append(data[:n+1], raw...)
	return
}

// InputStream represents a reader that can also be read byte by byte.
type InputStream interface {
	io.Reader
	io.ByteReader
}

// Unpack unpacks
func Unpack(in InputStream) (m proto.Message, err error) {
	// Read type identifier
	b, err := in.ReadByte()
	if err != nil {
		return
	}

	if b == 0 || b > byte(len(typeIdentifiers)) {
		err = proto.ErrInternalBadWireType
		return
	}

	mType := proto.MessageType(typeIdentifiers[b])
	m = reflect.New(mType.Elem()).Interface().(proto.Message)

	// Read length
	l, err := binary.ReadUvarint(in)
	if err != nil {
		return
	}

	// Unmarshal data
	if l > (1 << 30) {
		err = errors.New("invalid length")
		return
	}

	i := int(l)

	buf := make([]byte, i)
	_, err = io.ReadFull(in, buf)
	if err != nil {
		return
	}

	err = proto.Unmarshal(buf, m)
	return m, err
}
