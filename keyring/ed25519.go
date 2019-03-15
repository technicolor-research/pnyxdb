/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package keyring

import (
	"crypto/rand"

	"golang.org/x/crypto/ed25519"
)

func init() {
	cryptoEngines["ed25519"] = ed25519Engine{}
}

type ed25519Engine struct{}

func (ed25519Engine) Generate() (secret, public []byte, err error) {
	return ed25519.GenerateKey(rand.Reader)
}

func (ed25519Engine) Validate(public []byte) bool {
	return ed25519.PublicKeySize == len(public)
}

func (ed25519Engine) Sign(secret, cleartext []byte) []byte {
	return ed25519.Sign(secret, cleartext)
}

func (ed25519Engine) Verify(public, cleartext, signature []byte) bool {
	return ed25519.Verify(public, cleartext, signature)
}
