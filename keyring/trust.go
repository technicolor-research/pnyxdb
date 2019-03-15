/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package keyring

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// TrustLevel is a representation of a public key's trust.
type TrustLevel byte

// TrustLevel available values.
const (
	TrustNONE     TrustLevel = 0x00
	TrustLOW      TrustLevel = 0x01
	TrustHIGH     TrustLevel = 0x03
	TrustULTIMATE TrustLevel = 0xff
)

var trustName = map[TrustLevel]string{
	TrustNONE:     "none",
	TrustLOW:      "low",
	TrustHIGH:     "high",
	TrustULTIMATE: "ultimate",
}

// ParseTrust returns a TrustLevel from its string representation.
func ParseTrust(trust string) (TrustLevel, error) {
	trust = strings.ToLower(trust)
	for lvl, str := range trustName {
		if str == trust {
			return lvl, nil
		}
	}

	return TrustNONE, errors.New("unrecognized trust level")
}

func (t TrustLevel) String() string {
	str, ok := trustName[t]
	if ok {
		return str
	}

	return strconv.Itoa(int(t))
}

// Min returns the minimum value between two TrustLevels.
func (t TrustLevel) Min(t2 TrustLevel) TrustLevel {
	if t < t2 {
		return t
	}
	return t2
}

// Add returns a safe addition between two TrustLevels.
func (t TrustLevel) Add(t2 TrustLevel) TrustLevel {
	if t == TrustULTIMATE || t2 == TrustULTIMATE {
		return TrustULTIMATE
	}

	if t >= TrustThreshold || t2 >= TrustThreshold {
		return TrustThreshold
	}

	return t + t2
}

// TrustThreshold is the default required TrustLevel for a verification operation.
var TrustThreshold = TrustHIGH

const (
	pemPublicType  = "PNYXDB PUBLIC KEY"
	pemPrivateType = "PNYXDB PRIVATE KEY"
	pemCipher      = x509.PEMCipherAES256
)

// ListedKey shall contain one function returning basic informations about one's key.
type ListedKey interface {
	Info() (identity string, data []byte, trust TrustLevel)
}

// ByIdentity is a helper to sort ListeKey by their identity.
type ByIdentity []ListedKey

func (a ByIdentity) Len() int      { return len(a) }
func (a ByIdentity) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByIdentity) Less(i, j int) bool {
	ii, _, _ := a[i].Info()
	jj, _, _ := a[j].Info()
	return ii < jj
}

// Fingerprint is a helper function to get a human-friendly representation of one's key.
func Fingerprint(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	return strings.Replace(fmt.Sprintf("% X", data[len(data)-5:]), " ", ":", -1)
}
