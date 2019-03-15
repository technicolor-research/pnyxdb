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
	"crypto/sha512"
	"errors"
)

// ErrVersionMismatch is returned when two versions are not matching.
var ErrVersionMismatch = errors.New("the stored version does not match with required version")

// NoVersion is the default version that should be returned when no
// version is available in one store for a specific key.
var NoVersion = &Version{}

// VersionBytes is the space used by the version when marshalled.
const VersionBytes = sha512.Size

// NewVersion returns a new version from some data.
func NewVersion(data []byte) *Version {
	h := sha512.Sum512(data)
	return &Version{
		Hash: h[:],
	}
}

// Matches returns an error is two versions are not matching.
func (v *Version) Matches(v2 *Version) error {
	if v == nil || v2 == nil {
		return errors.New("only accepts non-nil version")
	}

	if !bytes.Equal(v.Hash, v2.Hash) {
		return ErrVersionMismatch
	}
	return nil
}

// MarshalBinary converts the version to a VersionBytes-sized bytes slice.
func (v *Version) MarshalBinary() (data []byte, err error) {
	if v == nil {
		return make([]byte, VersionBytes), nil
	}

	return v.Hash, nil
}

// UnmarshalBinary converts the input to a version.
func (v *Version) UnmarshalBinary(data []byte) error {
	if v == nil {
		return nil
	}

	v.Hash = make([]byte, len(data))
	copy(v.Hash, data)
	return nil
}
