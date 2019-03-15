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
	"encoding/gob"
	"errors"
	"io"
)

var dumpHeader = []byte(" PNYXDB_DUMP_V1 ")

// Dump stores the current state of an engine, to be later loaded with Load.
func (e *Engine) Dump(w io.Writer) error {
	return e.qs.Dump(w)
}

// Load loads the state of an engine from a dump file.
func (e *Engine) Load(r io.Reader) error {
	return e.qs.Load(r)
}

func (e *Engine) markActive() {
	select {
	case e.ActivityProbe <- true:
	default:
	}
}

func (qs *queryStore) Dump(w io.Writer) error {
	encoder := gob.NewEncoder(w)
	_, err := w.Write(dumpHeader)
	if err != nil {
		return err
	}

	qs.RLock()
	defer qs.RUnlock()

	err = encoder.Encode(qs.queries)
	if err != nil {
		return err
	}

	err = encoder.Encode(qs.pendingDependencies)
	if err != nil {
		return err
	}

	err = encoder.Encode(qs.pendingEndorsements)
	if err != nil {
		return err
	}

	return nil
}

func (qs *queryStore) Load(r io.Reader) error {
	initBuf := make([]byte, len(dumpHeader))
	_, err := io.ReadFull(r, initBuf)
	if err != nil {
		return err
	}

	if !bytes.Equal(initBuf, dumpHeader) {
		return errors.New("invalid dump header")
	}

	decoder := gob.NewDecoder(r)

	qs.Lock()
	defer qs.Unlock()

	err = decoder.Decode(&qs.queries)
	if err != nil {
		return err
	}

	err = decoder.Decode(&qs.pendingDependencies)
	if err != nil {
		return err
	}

	err = decoder.Decode(&qs.pendingEndorsements)
	if err != nil {
		return err
	}

	return nil
}
