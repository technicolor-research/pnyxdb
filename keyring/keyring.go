/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package keyring

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"sort"
	"sync"

	"github.com/awnumar/memguard"
)

// Signature represents a local or third-party public key's signature.
type Signature struct {
	Data  []byte
	Trust TrustLevel
}

type cryptoEngine interface {
	Generate() (secret, public []byte, err error)
	Validate(public []byte) bool
	Sign(secret, cleartext []byte) []byte
	Verify(public, cleartext, signature []byte) bool
}

var cryptoEngines = make(map[string]cryptoEngine)

// Key is the representation of a Key for the KeyRing.
type Key struct {
	Public     []byte
	Signatures map[string]*Signature

	identity       string
	signedBy       []*Key
	trust          TrustLevel // set by user
	effectiveTrust TrustLevel // computed from web of trust, >= trust
}

// Info shall be used to get basic informations about this key.
func (k *Key) Info() (identity string, data []byte, trust TrustLevel) {
	return k.identity, k.Public, k.trust
}

// KeyRing is a KeyRing saving data as PEM, and using the Ed25519
// high-speed high-security signatures algorithm.
//
// This KeyRing also provides a lazy web of trust computation feature,
// similar to PGP's web of trust.
type KeyRing struct {
	cryptoEngine

	selfIdentity  string
	mutex         sync.RWMutex
	keys          map[string]*Key
	secret        *memguard.LockedBuffer
	armoredSecret *pem.Block
	stale         bool
}

// NewKeyRing instanciates a new KeyRing.
// It MUST be called to create a new KeyRing.
//
// cryptoType may only be "ed25519" at the moment.
func NewKeyRing(selfIdentity string, crypto string) (*KeyRing, error) {
	ce, ok := cryptoEngines[crypto]
	if !ok {
		return nil, ErrUnknownCryptoEngine{CE: crypto}
	}

	return &KeyRing{
		cryptoEngine: ce,
		selfIdentity: selfIdentity,
		keys: map[string]*Key{
			selfIdentity: {
				identity:       selfIdentity,
				trust:          TrustULTIMATE,
				effectiveTrust: TrustULTIMATE,
				Signatures:     make(map[string]*Signature),
			},
		},
	}, nil
}

// Identity returns self identity
func (k *KeyRing) Identity() string {
	return k.selfIdentity
}

// Locked returns wether the KeyRing is currently locked or not (private key in cleartext in memory).
func (k *KeyRing) Locked() bool {
	return k.secret == nil
}

// LockPrivate locks the KeyRing by removing any remaining clear private key data in memory.
func (k *KeyRing) LockPrivate() (err error) {
	if k.Locked() {
		return // already locked
	}

	k.secret.Destroy()
	k.secret = nil
	return
}

// UnlockPrivate tries to decypher the private key block in memory.
func (k *KeyRing) UnlockPrivate(password *memguard.LockedBuffer) (err error) {
	if !k.Locked() {
		return // already unlocked
	}

	var secret []byte
	secret, err = x509.DecryptPEMBlock(k.armoredSecret, password.Buffer())
	if err != nil {
		return
	}

	k.secret, err = memguard.NewImmutableFromBytes(secret)
	return
}

// CreatePrivate generates a new Ed25519 private key and its associated PEM-armored block.
func (k *KeyRing) CreatePrivate(password *memguard.LockedBuffer) (err error) {
	var secret []byte
	k.keys[k.selfIdentity].Public, secret, err = k.Generate()
	if err != nil {
		return
	}

	k.secret, err = memguard.NewImmutableFromBytes(secret)
	if err != nil {
		return
	}

	// Generate private key PEM
	k.armoredSecret, err = x509.EncryptPEMBlock(
		rand.Reader,
		pemPrivateType,
		k.secret.Buffer(),
		password.Buffer(),
		pemCipher,
	)
	return
}

// GetPrivate returns a memguarded slice containing the raw private key.
// This can be useful when sharing a private key between several objects
// (for instance between a KeyRing and a consensus.Network)
func (k *KeyRing) GetPrivate() []byte {
	if k.secret == nil {
		return nil
	}

	return k.secret.Buffer()
}

// AddPublic adds or overwrite a new public key in the keyring.
// It resets the related signatures if the key is modified.
//
// This function is thread-safe.
func (k *KeyRing) AddPublic(identity string, trust TrustLevel, data []byte) (err error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if identity == k.selfIdentity {
		return ErrInvalidIdentity
	}

	if !k.Validate(data) {
		return ErrInvalidPublicKey
	}

	key, ok := k.keys[identity]
	if !ok {
		key = &Key{}
		k.keys[identity] = key
	}

	if !bytes.Equal(key.Public, data) {
		key.Public = make([]byte, len(data))
		key.Signatures = make(map[string]*Signature)
		copy(key.Public, data)
	}

	key.identity = identity
	key.trust = trust
	k.stale = true
	return
}

// ListPublic returns every stored public key.
// The self public key is also included.
func (k *KeyRing) ListPublic() []ListedKey {
	keys := make([]ListedKey, len(k.keys))
	var i int
	for _, key := range k.keys {
		keys[i] = key
		i++
	}

	sort.Sort(ByIdentity(keys))
	return keys
}

// GetPublic returns the stored public key for the provided identity.
// Providing the empty identity will return self public key.
//
// It may returns ErrKeyRingLocked or ErrUnknownIdentity.
//
// This function is thread-safe.
func (k *KeyRing) GetPublic(identity string) (data []byte, trust TrustLevel, err error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	key, ok := k.keys[identity]
	if !ok {
		err = &ErrUnknownIdentity{I: identity}
		return
	}

	trust = key.trust
	data = make([]byte, len(key.Public))
	copy(data, key.Public)

	return
}

// RemovePublic removes a key from the KeyRing.
// This function is thread-safe.
func (k *KeyRing) RemovePublic(identity string) {
	if identity == k.selfIdentity {
		return
	}

	k.mutex.Lock()
	defer k.mutex.Unlock()
	delete(k.keys, identity)
	k.stale = true
}

// Export exports a public key to a PEM block.
func (k *KeyRing) Export(identity string) ([]byte, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	_, ok := k.keys[identity]
	if !ok {
		return nil, &ErrUnknownIdentity{I: identity}
	}

	return k.exportUnsafe(identity)
}

func (k *KeyRing) exportUnsafe(identity string) ([]byte, error) {
	key := k.keys[identity]

	bytes, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}

	b := &pem.Block{
		Type: pemPublicType,
		Headers: map[string]string{
			"identity": key.identity,
			"trust":    key.trust.String(),
		},
		Bytes: bytes,
	}

	if key.identity == k.selfIdentity {
		b.Headers = map[string]string{}
	}

	return pem.EncodeToMemory(b), nil
}

// MarshalBinary returns a PEM-armored version of this KeyRing.
func (k *KeyRing) MarshalBinary() ([]byte, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	buf := pem.EncodeToMemory(k.armoredSecret)

	for identity := range k.keys {
		raw, err := k.exportUnsafe(identity)
		if err != nil {
			return nil, err
		}

		buf = append(buf, raw...)
	}

	return buf, nil
}

// Import imports a public PEM block to the keyring.
// Identity must be defined, and third-party signatures are verified afterwards.
//
// This function accepts following results of function Export:
// - Local exports (without any headears)
// - Third-party exports (with "identity" header set)
//   * If the provided identity is different that the "identity" header, an error is returned
//
// This function is thread-safe.
func (k *KeyRing) Import(data []byte, identity string, trust TrustLevel) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if identity == k.selfIdentity || identity == "" {
		return ErrInvalidIdentity
	}

	_, err := k.importUnsafe(data, identity, trust)
	return err
}

func (k *KeyRing) importUnsafe(data []byte, identity string, trust TrustLevel) (remaining []byte, err error) {
	block, remaining := pem.Decode(data)

	if block == nil {
		err = io.EOF
		return
	}

	if block.Type == pemPrivateType {
		if identity != "" && identity != k.selfIdentity { // Avoid private key override when importing unsafely.
			err = ErrInvalidIdentity
			return
		}
		k.armoredSecret = block
	} else if block.Type == pemPublicType {
		lvl, _ := ParseTrust(block.Headers["trust"]) // error is handled by the default lvl value
		id := block.Headers["identity"]

		key := &Key{
			identity: id,
			trust:    lvl,
		}

		err = json.Unmarshal(block.Bytes, key)
		if err != nil {
			err = ErrInvalidSignature
			return
		}

		if identity != "" {
			if key.identity != "" && key.identity != identity {
				err = ErrInvalidIdentity
				return
			}

			key.identity = identity
			key.trust = trust
		} else if key.identity == "" { // identity == "" and key.identity == ""
			key.identity = k.selfIdentity
			key.trust = TrustULTIMATE
		}

		k.keys[key.identity] = key
	}

	k.stale = true
	return remaining, nil
}

// UnmarshalBinary rebuilds a KeyRing from its PEM-armored version.
// - It may not return an error if a parse error is encountered ;
// - NewKeyRing must be called before to instantiate the KeyRing.
func (k *KeyRing) UnmarshalBinary(data []byte) error {
	var err error
	buffer := data

	for len(buffer) > 0 && err != io.EOF {
		buffer, err = k.importUnsafe(buffer, "", 0)
	}

	return nil
}
