/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package keyring provides security primitives for PnyxDB consortium networks.
package keyring

import (
	"errors"
	"fmt"
)

// Error messages.
var (
	ErrKeyRingLocked    = errors.New("keyring is locked")
	ErrInvalidIdentity  = errors.New("invalid identity")
	ErrInvalidPublicKey = errors.New("invalid public key")
	ErrInvalidSignature = errors.New("invalid signature")
)

// ErrUnknownIdentity is returned when an operation is asked for an unknown identity.
type ErrUnknownIdentity struct {
	I string
}

// Error returns error's string value.
func (e ErrUnknownIdentity) Error() string {
	return "unknown identity: " + e.I
}

// ErrInsufficientTrust is returned when a verification cannot be performed due to a lack of trust in one's public key.
type ErrInsufficientTrust struct {
	I string
	L int
}

// Error returns error's string value.
func (e ErrInsufficientTrust) Error() string {
	return fmt.Sprintf("insufficient trust for identity %s (%d/%d)", e.I, e.L, TrustThreshold)
}

// ErrUnknownCryptoEngine is returned when an operation requires an unknown crypto engine.
type ErrUnknownCryptoEngine struct {
	CE string
}

func (e ErrUnknownCryptoEngine) Error() string {
	return "unknown crypto engine: " + e.CE
}
