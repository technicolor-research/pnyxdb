/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package keyring

// GetSignatures returns a map of (signer, signatures) where the provided identity is the signee.
// This function is thread-safe.
func (k *KeyRing) GetSignatures(identity string) map[string]*Signature {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	k.waitForStaleCleared()

	key, ok := k.keys[identity]
	if !ok {
		return nil
	}

	// Copy map
	signatures := make(map[string]*Signature)
	for _, signer := range key.signedBy {
		signatures[signer.identity] = signer.Signatures[identity]
	}

	return signatures
}

// AddSignature adds a signature to the identity, from signer "from".
// If "from" equals the k.selfIdentity, the KeyRing adds a new signature to the identity using its own private key.
//
// It may returns ErrKeyRingLocked or ErrUnknownIdentity.
//
// This function is thread-safe.
func (k *KeyRing) AddSignature(identity, from string, signature *Signature) error {
	k.mutex.RLock()
	key, ok := k.keys[identity]
	signer, ok2 := k.keys[from]
	k.mutex.RUnlock()

	if !ok {
		return &ErrUnknownIdentity{I: identity}
	}

	if !ok2 {
		return &ErrUnknownIdentity{I: from}
	}

	if from == k.selfIdentity { // emit local signature
		message := append(key.Public, byte(key.trust))
		signData, err := k.Sign(message)
		if err != nil {
			return err
		}

		signature = &Signature{
			Data:  signData,
			Trust: key.trust,
		}
	} else if err := k.verifySignature(from, key, signature); err != nil {
		// verify third-party signature
		return err
	}

	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.stale = true
	signer.Signatures[identity] = signature
	return nil
}

// Sign signs the message with the unlocked private key.
// This function is thread-safe.
func (k *KeyRing) Sign(cleartext []byte) (signature []byte, err error) {
	if k.Locked() {
		err = ErrKeyRingLocked
		return
	}

	signature = k.cryptoEngine.Sign(k.secret.Buffer(), cleartext)
	return
}

// Verify checks the message signed by "from".
// The addition of local trust and third-party trust levels must be greater or equals than TrustThreshold.
//
// It may returns ErrUnknownIdentity, ErrInsufficientTrust or ErrInvalidSignature.
//
// This function is thread-safe.
func (k *KeyRing) Verify(from string, cleartext, signature []byte) error {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	k.waitForStaleCleared()

	key, ok := k.keys[from]
	if !ok {
		return &ErrUnknownIdentity{I: from}
	}

	if !k.cryptoEngine.Verify(key.Public, cleartext, signature) {
		return ErrInvalidSignature
	}

	return k.trustedUnsafe(key)
}

// Verify signature does NOT check for trust chain.
// It only checks that a signature fulfill cryptographic requirements.
func (k *KeyRing) verifySignature(signer string, signee *Key, signature *Signature) error {
	message := append(signee.Public, byte(signature.Trust))
	if !k.cryptoEngine.Verify(k.keys[signer].Public, message, signature.Data) {
		return ErrInvalidSignature
	}
	return nil
}

// Trusted shall return nil if an identity is currently trusted by the keyring.
//
// It may returns ErrUnknownIdentity or ErrInsufficientTrust.
//
// This function is thread-safe.
func (k *KeyRing) Trusted(identity string) error {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	k.waitForStaleCleared()

	key, ok := k.keys[identity]
	if !ok {
		return &ErrUnknownIdentity{I: identity}
	}

	return k.trustedUnsafe(key)
}

// This function MUST me called by other functions that hold a read-only
// lock against the KeyRing, and wish to clear the staled state.
func (k *KeyRing) waitForStaleCleared() {
	for k.stale {
		k.mutex.RUnlock()
		k.mutex.Lock()
		if k.stale {
			k.buildTrustWeb()
		}
		k.mutex.Unlock()
		k.mutex.RLock()
	}
}

func (k *KeyRing) trustedUnsafe(key *Key) error {
	if key.effectiveTrust < TrustThreshold {
		return &ErrInsufficientTrust{
			I: key.identity,
			L: int(key.effectiveTrust),
		}
	}
	return nil
}

// buildTrustWeb constructs the Web of Trust.
// As such, it is a critical part of the KeyRing.
//
// It works by performing a greedy BFS algorithm in the
// peer directed graph. This strategy is used because we
// need to iteratively trust more and more peers.
//
// This function is not thread-safe and is called internally
// when the KeyRing is considered stale.
func (k *KeyRing) buildTrustWeb() {
	var queue []*Key
	visited := make(map[string]bool)

	// Populate initial trusted peers.
	// The queue only contains peers whose signatures can be trusted.
	for _, key := range k.keys {
		if key.trust >= TrustThreshold {
			queue = append(queue, key)
			visited[key.identity] = true
		}

		key.effectiveTrust = key.trust
		key.signedBy = nil
	}

	// While there are some vertexes to be processed
	var current *Key
	for len(queue) > 0 {
		current, queue = queue[0], queue[1:]

		// For each signatures
		for signee, signature := range current.Signatures {

			// The signature is valid, add its value (if exists)
			signeeKey := k.keys[signee]
			if signeeKey == nil {
				continue
			}

			// EffectiveTrust calculation takes into account previously
			// accumulated trust wrt signer's trust.
			signeeKey.effectiveTrust = signeeKey.effectiveTrust.Add(
				signature.Trust.Min(current.effectiveTrust),
			)
			signeeKey.signedBy = append(signeeKey.signedBy, current)

			// Is it the first time we can trust the signee?
			if signeeKey.effectiveTrust >= TrustThreshold {
				if !visited[signee] {
					queue = append(queue, signeeKey)
					visited[signee] = true
				}
			}
		}
	}

	k.stale = false
}
