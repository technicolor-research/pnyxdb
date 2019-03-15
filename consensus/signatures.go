/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import "github.com/bluele/gcache"

func (eng *Engine) verifyQuery(q *Query) error {
	hash, err := eng.hashes.GetIFPresent(q.Uuid)
	if err == gcache.KeyNotFoundError {
		hash, err = q.Hash()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	err = eng.KeyRing.Verify(q.Emitter, hash.([]byte), q.Signature)
	if err != nil {
		return err
	}

	_ = eng.hashes.Set(q.Uuid, hash)
	return nil
}

func (eng *Engine) signQuery(q *Query) error {
	hash, err := q.Hash()
	if err != nil {
		return err
	}

	q.Signature, err = eng.KeyRing.Sign(hash)
	return err
}

func (eng *Engine) verifyEndorsement(e *Endorsement) error {
	hash, err := e.Hash()
	if err != nil {
		return err
	}

	return eng.KeyRing.Verify(e.Emitter, hash, e.Signature)
}

func (eng *Engine) signEndorsement(e *Endorsement) error {
	hash, err := e.Hash()
	if err != nil {
		return err
	}

	e.Signature, err = eng.KeyRing.Sign(hash)
	return err
}
