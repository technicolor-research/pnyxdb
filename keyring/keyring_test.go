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
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"io"
	"strings"
	"testing"

	"github.com/awnumar/memguard"
	"github.com/stretchr/testify/require"
)

const selfIdentity = "default"

const testPEMPrivateKeyRing = `-----BEGIN PNYXDB PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,c17d3a85686217f7ad7e6a3de99a47ef

NLMYWwAQd6zfjuXVyx1OKEOjoSLp3wVXPj38NRfxCj7DwjNC0oVURVhwwb3eL/LP
5HNxbNKAGLVr5LwtqyVy6qIsd/bco31ld6gTQdXFHcw=
-----END PNYXDB PRIVATE KEY-----
`

type testKeyPairKeyRing struct {
	sec, pub string
}

var testKeyPairsKeyRing = []testKeyPairKeyRing{
	{
		"f5cbfc7e538568293bb21e7cfbe9b0e91e5071e0a93b74a8721cd6f8bcd51b65" +
			"62c677aefb173821269ead5e91dc6c7c888ba6a8908d2dabf21902f7d8706b5a",
		"62c677aefb173821269ead5e91dc6c7c888ba6a8908d2dabf21902f7d8706b5a",
	},
	{
		"8bb645bce494df8498687f9345ce9e9d050ff43b80c792519eb4d8a8844c4f05" +
			"72acc39d3ae6c2c73e28a88c166273c97138a334b4c35eb32dddd7e95d427eb8",
		"72acc39d3ae6c2c73e28a88c166273c97138a334b4c35eb32dddd7e95d427eb8",
	},
	{
		"d45200488b20e8a215dcd06ec88b60da340a955d99aef16312c49c2c0e44da59" +
			"768b9db78bdc37b98a6fe9a685af85a33a28f5ac37d4c5aea4f12881f9d13650",
		"768b9db78bdc37b98a6fe9a685af85a33a28f5ac37d4c5aea4f12881f9d13650",
	},
	{
		"2ab4b55cc6cbb333931da826643d4b08fb535aef9bf420231fab3758d30ca0c6" +
			"f9d930b2aba2e83fceecd7b3e793fb01f26c66706a1f03c4b1bb39a079f0ed6f",
		"f9d930b2aba2e83fceecd7b3e793fb01f26c66706a1f03c4b1bb39a079f0ed6f",
	},
}

func getTestPubKeyRing(i int) []byte {
	raw, _ := hex.DecodeString(testKeyPairsKeyRing[i].pub)
	return raw
}

func getTestSecKeyRing(i int) *memguard.LockedBuffer {
	raw, _ := hex.DecodeString(testKeyPairsKeyRing[i].sec)
	buf, _ := memguard.NewImmutableFromBytes(raw)
	return buf
}

func TestKeyRing_UnlockPrivate(t *testing.T) {
	k, err := NewKeyRing(selfIdentity, "ed25519")
	require.Nil(t, err)
	k.armoredSecret, _ = pem.Decode([]byte(testPEMPrivateKeyRing))

	wrongPass, _ := memguard.NewImmutableFromBytes([]byte("wrong"))
	defer wrongPass.Destroy()

	rightPass, _ := memguard.NewImmutableFromBytes([]byte("password"))
	defer rightPass.Destroy()

	require.NotNil(t, k.UnlockPrivate(wrongPass))
	require.Nil(t, k.UnlockPrivate(rightPass))
	require.NotNil(t, k.secret)
}

func TestEd22519_CreatePrivate(t *testing.T) {
	password, _ := memguard.NewImmutableFromBytes([]byte("password"))
	defer password.Destroy()

	k, err := NewKeyRing(selfIdentity, "ed25519")
	require.Nil(t, err)
	err = k.CreatePrivate(password)
	require.Nil(t, err)

	armor := string(pem.EncodeToMemory(k.armoredSecret))
	prefix := `-----BEGIN PNYXDB PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC`

	require.True(t, strings.HasPrefix(armor, prefix))
}

func TestKeyRing_AddGetPublic(t *testing.T) {
	defer memguard.DestroyAll()

	k, err := NewKeyRing(selfIdentity, "ed25519")
	require.Nil(t, err)
	k.secret = getTestSecKeyRing(0)

	// Test invalid key
	err = k.AddPublic("wrong", TrustHIGH, []byte("HELLO"))
	require.Exactly(t, ErrInvalidPublicKey, err)

	// Test unknown identity
	_, _, err = k.GetPublic("wrong")
	require.NotNil(t, err)

	// Test valid key
	expected := getTestPubKeyRing(1)
	require.Nil(t, k.AddPublic("a", TrustHIGH, expected))
	got, trust, err := k.GetPublic("a")
	require.Nil(t, err)
	require.Exactly(t, expected, got)
	require.Exactly(t, TrustHIGH, trust)

	// Test overwrite existing key
	require.Nil(t, k.AddPublic("a", TrustLOW, expected))
	got, trust, err = k.GetPublic("a")
	require.Nil(t, err)
	require.Exactly(t, expected, got)
	require.Exactly(t, TrustLOW, trust)

	// Test invalid identity
	require.Exactly(t, ErrInvalidIdentity, k.AddPublic(selfIdentity, TrustHIGH, getTestPubKeyRing(0)))
}

func TestKeyRing_SignVerify(t *testing.T) {
	defer memguard.DestroyAll()

	k1, _ := NewKeyRing("todo", "ed25519")
	k1.secret = getTestSecKeyRing(0)

	k2, _ := NewKeyRing("todo", "ed25519")
	k2.stale = true
	k2.keys["k1"] = &Key{
		Public:   getTestPubKeyRing(0),
		identity: "k1",
		trust:    TrustULTIMATE,
	}

	key2 := &Key{
		Public:   getTestPubKeyRing(1),
		identity: "k2",
		trust:    TrustULTIMATE,
		Signatures: map[string]*Signature{
			"k1": {
				Trust: TrustULTIMATE,
			},
		},
	}
	k3 := &KeyRing{
		cryptoEngine: k2.cryptoEngine,
		keys: map[string]*Key{
			"k1": {
				Public:   getTestPubKeyRing(0),
				identity: "k1",
				trust:    TrustNONE,
			},
			"k2": key2,
		},
		stale: true,
	}

	message := []byte("Hello World!")
	signature, err := k1.Sign(message)
	require.Nil(t, err)

	type tc struct {
		name, identity     string
		message, signature []byte
		err                bool
	}

	cases := []*tc{
		{"valid", "k1", message, signature, false},
		{"unknown", "A", message, signature, true},
		{"invalid_message", "k1", append(message, 0x00), signature, true},
		{"invalid_signature", "k1", message, append([]byte("A"), signature[1:]...), true},
		{"bad_length_signature", "k1", message, []byte("AA"), true},
	}

	for name, verifier := range map[string]*KeyRing{
		"ULTIMATE": k2,
		"PARENT":   k3,
	} {
		v := verifier
		t.Run(name, func(t *testing.T) {
			for _, tc := range cases {
				c := tc
				t.Run(c.name, func(t *testing.T) {
					err := v.Verify(c.identity, c.message, c.signature)
					if c.err {
						require.NotNil(t, err)
					} else {
						require.Nil(t, err)
					}
				})
			}
		})
	}
}

func TestKeyRing_AddGetSignature(t *testing.T) {
	defer memguard.DestroyAll()

	// Scenario: k0 will sign 2's identity and give it to k1.
	k0, _ := NewKeyRing("k0", "ed25519")
	k0.secret = getTestSecKeyRing(0)

	k1, _ := NewKeyRing("k1", "ed25519")
	k1.secret = getTestSecKeyRing(1)

	require.Nil(t, k0.AddPublic("k2", TrustHIGH, getTestPubKeyRing(2)))
	require.Nil(t, k1.AddPublic("k0", TrustULTIMATE, getTestPubKeyRing(0)))

	require.Nil(t, k1.GetSignatures("k2"), "not yet registered")

	require.Nil(t, k1.AddPublic("k2", TrustNONE, getTestPubKeyRing(2)))
	require.Len(t, k1.GetSignatures("k2"), 0, "not yet signed by third parties")

	require.NotNil(t, k1.AddSignature("k3", "k1", &Signature{}), "should not accept unknown signee")
	require.NotNil(t, k1.AddSignature("k0", "k3", &Signature{}), "should not accept unknown signer")

	require.Nil(t, k1.AddSignature("k2", "k1", nil))
	signatures := k1.GetSignatures("k2")
	require.Len(t, signatures, 1, "expect exactly one signature")
	require.Exactly(t, TrustNONE, signatures["k1"].Trust)

	err := k0.AddSignature("k2", "k0", nil)
	require.Nil(t, err)
	signatures = k0.GetSignatures("k2")

	s := &Signature{
		Trust: TrustULTIMATE,
		Data:  signatures["k0"].Data,
	}
	require.NotNil(t, k1.AddSignature("k2", "k0", s), "should not accept invalid signatures")
	require.Nil(t, k1.AddSignature("k2", "k0", signatures["k0"]), "should accept valid signatures")

	signatures = k1.GetSignatures("k2")
	require.Len(t, signatures, 2, "expect exactly two signatures")
	require.NotNil(t, signatures["k0"])
	require.NotNil(t, signatures["k0"])
}

func TestKeyRing_Export(t *testing.T) {
	k, _ := NewKeyRing(selfIdentity, "ed25519")
	password, _ := memguard.NewImmutableFromBytes([]byte("password"))
	defer password.Destroy()

	_ = k.CreatePrivate(password)

	data, err := k.Export(selfIdentity)
	require.Nil(t, err)
	require.True(t, strings.HasPrefix(string(data), "-----BEGIN "))

	_, err = k.Export("unknown")
	require.NotNil(t, err)
}

func TestKeyRing_Marshal(t *testing.T) {
	password, _ := memguard.NewImmutableFromBytes([]byte("password"))
	defer memguard.DestroyAll()

	k0, _ := NewKeyRing("k0", "ed25519")
	k0.secret = getTestSecKeyRing(0)
	k0.keys["k0"].Public = getTestPubKeyRing(0)
	_ = k0.AddPublic("k1", TrustHIGH, getTestPubKeyRing(1))
	_ = k0.AddPublic("k2", TrustLOW, getTestPubKeyRing(2))
	_ = k0.AddSignature("k2", "k0", nil)

	k1, _ := NewKeyRing("k1", "ed25519")
	k1.secret = getTestSecKeyRing(1)
	k1.keys["k1"].Public = getTestPubKeyRing(1)
	k1.armoredSecret, _ = x509.EncryptPEMBlock(
		rand.Reader,
		pemPrivateType,
		k1.secret.Buffer(),
		password.Buffer(),
		pemCipher,
	)
	_ = k1.AddPublic("k0", TrustHIGH, getTestPubKeyRing(0))
	_ = k1.AddPublic("k2", TrustNONE, getTestPubKeyRing(2))
	_ = k1.AddSignature("k0", "k1", nil)
	_ = k1.AddSignature("k2", "k0", k0.GetSignatures("k2")["k0"])

	data, err := k1.MarshalBinary()
	require.Nil(t, err)
	require.True(t, strings.HasPrefix(string(data), "-----BEGIN "))
}

var armoredTestKeyRing = []string{
	`-----BEGIN PNYXDB PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,7d7838f01f3a9a47dea053bbe7f38140

GXzO2y1auLrbCxD4N+hErrQnOGIJJ7uVS17UHn3cB+nEhAV2CXMIHbc38Pf/p+sy
BMWAo1MeLvYvWQTwMmoWwRvaVvdOrBFbMj5kxkRC5Qg=
-----END PNYXDB PRIVATE KEY-----
`, `-----BEGIN PNYXDB PUBLIC KEY-----
eyJQdWJsaWMiOiJjcXpEblRybXdzYytLS2lNRm1KenlYRTRvelMwdzE2ekxkM1g2
VjFDZnJnPSIsIlNpZ25hdHVyZXMiOnsiazAiOnsiRGF0YSI6Ik05YkxESU1Ea2lY
QnFMV3FuOU41eGxKd1VZTzZDdnN3T1pwZmpoRE55S25waXVSdWUzWmZJak1aVm12
VTBXMUpVaEJmMmF6dVkzTVlSNWFEVFkyRUNRPT0iLCJUcnVzdCI6M319fQ==
-----END PNYXDB PUBLIC KEY-----
`, `-----BEGIN PNYXDB PUBLIC KEY-----
identity: k0
trust: high

eyJQdWJsaWMiOiJZc1ozcnZzWE9DRW1ucTFla2R4c2ZJaUxwcWlRalMycjhoa0M5
OWh3YTFvPSIsIlNpZ25hdHVyZXMiOnsiazIiOnsiRGF0YSI6IlhhcmJHNlNoSlFX
ZlpwMmZQVkFueGpiOUFkMk5PUldWUE84Q1pNN2I0NEFPTDhCZmJIWDBwSnBENGhQ
QjFtS3ZqVUNpS3V6OFFXNjdMc3RrT1RVVEJRPT0iLCJUcnVzdCI6MX19fQ==
-----END PNYXDB PUBLIC KEY-----
`, `-----BEGIN PNYXDB PUBLIC KEY-----
-----END PNYXDB PUBLIC KEY-----
`, `-----BEGIN PNYXDB PUBLIC KEY-----
identity: k2
trust: none

eyJQdWJsaWMiOiJkb3VkdDR2Y043bUtiK21taGErRm96b285YXczMU1XdXBQRW9n
Zm5STmxBPSIsIlNpZ25hdHVyZXMiOnt9fQ==
-----END PNYXDB PUBLIC KEY-----
`, `INVALID`}

var armoredTestKeyRingJoined = strings.Join(armoredTestKeyRing, "")

func TestKeyRing_Import(t *testing.T) {
	type tc struct {
		name     string
		data     int
		identity string
		trust    TrustLevel
		err      error
	}

	cases := []*tc{
		{"locally exported", 1, "k0", 1, nil},
		{"third-party exported", 2, "k0", 1, nil},
		{"third-party exported wrong identity", 2, "k1", 1, ErrInvalidIdentity},
		{"invalid PEM", 5, "k0", 1, io.EOF},
		{"invalid JSON", 3, "k0", 1, ErrInvalidSignature},
		{"private", 0, "k0", 1, ErrInvalidIdentity},
	}

	for _, tc := range cases {
		c := tc
		t.Run(c.name, func(t *testing.T) {
			k, _ := NewKeyRing("k1", "ed25519")
			require.Exactly(t, c.err, k.Import([]byte(armoredTestKeyRing[c.data]), c.identity, c.trust))
		})
	}
}

func TestKeyRing_Unmarshal(t *testing.T) {
	password, _ := memguard.NewImmutableFromBytes([]byte("password"))
	defer password.Destroy()

	k, _ := NewKeyRing("k1", "ed25519")
	require.Nil(t, k.UnmarshalBinary([]byte(armoredTestKeyRingJoined)))

	require.Nil(t, k.UnlockPrivate(password), "should retrieve correct password")

	data, trust, err := k.GetPublic("k0")
	require.Nil(t, err, "should retrieve k0's data")
	require.Exactly(t, getTestPubKeyRing(0), data, "should retrive k0's public key")
	require.Exactly(t, TrustHIGH, trust, "should retrive k0's local trust level")

	signatures := k.GetSignatures("k0")
	require.NotNil(t, signatures["k1"], "should retrieve local signatures")
	require.Exactly(t, signatures["k1"].Trust, TrustHIGH, "should retrieve local trust levels in signatures")

	signatures = k.GetSignatures("k2")
	require.NotNil(t, signatures["k0"], "should retrieve third-party signatures")
	require.Exactly(t, signatures["k0"].Trust, TrustLOW, "should retrieve local trust levels in third-party signatures")
}

func TestKeyRing_RemovePublic(t *testing.T) {
	k, _ := NewKeyRing(selfIdentity, "ed25519")
	_ = k.UnmarshalBinary([]byte(armoredTestKeyRingJoined))

	k.RemovePublic(selfIdentity)
	k.RemovePublic("k3")
	k.RemovePublic("k0")

	require.NotNil(t, k.keys[selfIdentity], "should not remove self key")
	require.NotNil(t, k.keys["k2"], "should not remove k2's key")
	require.Nil(t, k.keys["k0"], "must remove k0's key")

	signatures := k.GetSignatures("k2")
	require.Len(t, signatures, 0, "must remove related signatures")
}
