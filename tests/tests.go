/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package tests contains test utilities and scenarios.
//
// It shall not be included in the final package.
package tests

import (
	"strconv"
	"testing"
	"time"

	"github.com/awnumar/memguard"
	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/keyring"
)

// GetTestKeyRings returns a number of keyrings that trust each other.
func GetTestKeyRings(t *testing.T, n int) []*keyring.KeyRing {
	t.Log("Starting keyring generation...")
	start := time.Now()

	// Build keyrings
	keyrings := make([]*keyring.KeyRing, n)
	password, _ := memguard.NewImmutableRandom(16)
	for i := 0; i < n; i++ {
		keyrings[i], _ = keyring.NewKeyRing(strconv.Itoa(i), "ed25519")
		_ = keyrings[i].CreatePrivate(password)
	}

	// Build basic webtrust
	for i := 0; i < n; i++ {
		pub, _, _ := keyrings[i].GetPublic(keyrings[i].Identity())
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			err := keyrings[j].AddPublic(keyrings[i].Identity(), keyring.TrustHIGH, pub)
			require.Nil(t, err)
		}
	}

	t.Log("Finished keyring generation in", time.Since(start))
	return keyrings
}
