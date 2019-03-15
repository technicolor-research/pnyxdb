/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package bbc implements a very simple Byzantine Broadcast Consensus algorithm.
//
// This BBC algorithm supports the Veto procedure, as expected by main consensus.
package bbc

import (
	"context"
	"crypto/sha512"

	"github.com/golang/protobuf/proto"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/keyring"
)

type vetoEngine struct {
	*keyring.KeyRing

	n         consensus.Network
	threshold int
}

// NewVetoEngine returns a BBCEngine that works as a BV-broadcast
// algorithm, introduced by Mostéfaoui et al. in Signature-Free
// Asynchronous Binary Byzantine Consensus (ACM 2015) with a Veto
// variant.
func NewVetoEngine(n consensus.Network, k *keyring.KeyRing, threshold int) (consensus.BBCEngine, error) {
	return &vetoEngine{
		KeyRing:   k,
		n:         n,
		threshold: threshold,
	}, nil
}

func (ve vetoEngine) Execute(
	ctx context.Context,
	id string,
	choice bool,
	proofs []*consensus.Proof,
) (decision bool, dp []*consensus.Proof, err error) {
	c := &Choice{
		Identifier: id,
		Emitter:    ve.Identity(),
		Choice:     choice,
		Proofs:     proofs,
	}

	hash, err := c.Hash()
	if err != nil {
		return
	}

	c.Signature, err = ve.KeyRing.Sign(hash)
	if err != nil {
		return
	}

	err = ve.n.Broadcast(c)
	if err != nil {
		return
	}

	sentF := !choice
	receivedT := make(map[string]bool)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	acceptor := func(m proto.Message) bool {
		c, ok := m.(*Choice)
		return ok && c.Identifier == id
		// TODO verify proofs
	}

	for m := range ve.n.Accept(ctx, acceptor) {
		c := m.(*Choice)
		hash, err = c.Hash()
		if err != nil {
			continue
		}

		err = ve.KeyRing.Verify(c.Emitter, hash, c.Signature)
		if err != nil {
			continue
		}

		if !c.Choice {
			if !sentF {
				err = ve.n.Broadcast(c)
				if err == nil {
					sentF = true
				}
			}

			decision = false
			return decision, c.Proofs, nil
		}

		receivedT[c.Emitter] = true
		if len(receivedT) == ve.threshold { // Threshold reached
			break
		}
	}

	return true, nil, nil
}

// Hash returns a fixed-size hash of the (unsigned) version of the choice
// Passed by value because of internal modifications.
func (c Choice) Hash() ([]byte, error) {
	c.Signature = nil
	raw, err := proto.Marshal(&c)
	hash := sha512.Sum512(raw)
	return hash[:], err
}
