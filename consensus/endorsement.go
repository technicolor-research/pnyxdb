/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"crypto/sha512"

	"github.com/golang/protobuf/proto"
)

// Hash returns a fixed-size hash of the (unsigned) version of the endorsement.
// Passed by value because of internal modifications.
func (e Endorsement) Hash() ([]byte, error) {
	e.Signature = nil
	raw, err := proto.Marshal(&e)
	hash := sha512.Sum512(raw)
	return hash[:], err
}
