/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package consensus provides the main BFT consensus algorithm.
package consensus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bluele/gcache"
	"github.com/golang/protobuf/proto"
	"github.com/technicolor-research/pnyxdb/consensus/operations"
	"github.com/technicolor-research/pnyxdb/keyring"
	"go.uber.org/zap"
)

const loopDuration = 100 * time.Millisecond
const checkpointRoutineTimeout = 3 * time.Second
const checkpointRoutineBatch = 100
const checkpointRoutineSelect = 30
const checkpointRoutineCooldown = 100 * time.Millisecond // limit checkpoints to 10 requests / sec max

// Engine is the main consensus engine that can process queries and endorsements
type Engine struct {
	Store
	Network
	BBCEngine
	*keyring.KeyRing

	ctx                context.Context
	qs                 *queryStore
	checkpoints        gcache.Cache
	hashes             gcache.Cache
	quorum             int // minimum number of endorsement required for applicable state
	endorsementMutex   sync.Mutex
	pendingCheckpoints chan string
	pendingRecovery    chan string
	ActivityProbe      chan bool // will receive data when some activity requires persistence
}

// NewEngine TODO
func NewEngine(s Store, n Network, bbc BBCEngine, k *keyring.KeyRing, q int) *Engine {
	qs := newQueryStore()
	qs.threshold = q
	return &Engine{
		Store:              s,
		Network:            n,
		BBCEngine:          bbc,
		KeyRing:            k,
		qs:                 qs,
		checkpoints:        gcache.New(1024).LRU().Build(),
		hashes:             gcache.New(1024).LFU().Build(),
		quorum:             q,
		pendingCheckpoints: make(chan string, 1024),
		pendingRecovery:    make(chan string, 1024),
		ActivityProbe:      make(chan bool, 1),
	}
}

// Submit submits a new query to the network of processes.
func (eng *Engine) Submit(q *Query) error {
	q.Emitter = eng.KeyRing.Identity()
	err := eng.signQuery(q)
	if err != nil {
		return err
	}

	zap.L().Debug("Submit",
		zap.String("uuid", q.Uuid),
	)

	err = eng.Network.Broadcast(q)
	if err == nil {
		go eng.handleQuery(q)
	}
	return err
}

// Run starts the engine in a non-blocking way.
func (eng *Engine) Run(ctx context.Context) error {
	eng.ctx = ctx
	go func() {
		acceptor := func(m proto.Message) bool {
			_, ok := m.(*Query)
			return ok
		}

		for m := range eng.Network.Accept(ctx, acceptor) {
			go eng.handleQuery(m.(*Query))
		}
	}()

	go func() {
		acceptor := func(m proto.Message) bool {
			_, ok := m.(*Endorsement)
			return ok
		}

		for m := range eng.Network.Accept(ctx, acceptor) {
			eng.handleEndorsement(m.(*Endorsement))
		}
	}()

	go func() {
		acceptor := func(m proto.Message) bool {
			_, ok := m.(*StartCheckpoint)
			return ok
		}

		for m := range eng.Network.Accept(ctx, acceptor) {
			eng.handleCheckpoint(ctx, m.(*StartCheckpoint))
		}
	}()

	go func() {
		timer := time.NewTimer(checkpointRoutineTimeout)
		var pending []string

		start := func(expired bool) {
			if len(pending) > 0 {
				// Sort by id and submit only the first
				sort.Strings(pending)
				n := checkpointRoutineSelect
				if len(pending) < n {
					n = len(pending)
				}

				_ = eng.Network.Broadcast(&StartCheckpoint{Queries: pending[:n]})
				pending = pending[n:]
				zap.L().Debug("Checkpoint",
					zap.String("state", "pool"),
					zap.Int("sent", n),
					zap.Int("remaining", len(pending)),
				)

				// Introduce some arbitrary cooldown to avoid network contention
				time.Sleep(checkpointRoutineCooldown)
			}

			if !expired && !timer.Stop() {
				<-timer.C
			}
			timer.Reset(checkpointRoutineTimeout)
		}

		for {
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			case c := <-eng.pendingCheckpoints:
				pending = addToSet(pending, c)
				if len(pending) == checkpointRoutineBatch {
					start(false)
				}
			case <-timer.C:
				start(true)
			}
		}
	}()

	// Garbage collection mechanism
	// TODO optimize
	go func() {
		var i int
		for {
			i++
			select {
			case <-time.After(100 * time.Millisecond):
				if false && i == 5 { // TODO check this experimental attempt
					i = 0
					out := eng.qs.OutdatedQueries()
					for _, c := range out {
						select {
						case eng.pendingCheckpoints <- c:
						case <-ctx.Done():
							return
						}
					}
				} else {
					for _, uuid := range eng.qs.PendingQueries() {
						eng.checkState(uuid)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	rec, ok := eng.Network.(RecoveryManager)
	if ok {
		rec.AcceptRecovery(ctx, eng.recoveryHandler)
		zap.L().Info("Recovery", zap.String("handler", "ready"))
	}
	go eng.recoveryWorker(ctx)

	return nil
}

func (eng *Engine) handleQuery(q *Query) {
	err := eng.verifyQuery(q)
	if err != nil {
		zap.L().Warn("Invalid query",
			zap.String("uuid", q.Uuid),
			zap.Error(err),
		)
		return
	}

	inserted := eng.qs.AddQuery(q)
	if !inserted {
		return
	}

	defer eng.markActive()
	eng.checkState(q.Uuid)

	for {
		eng.endorsementMutex.Lock()
		if eng.canEndorse(q) {
			conflictingQueries := eng.qs.GetConflicting(q)
			if len(conflictingQueries) == 0 {
				eng.endorse(q, nil)
				eng.endorsementMutex.Unlock()
				return
			}

			allExpired := true
			for _, c := range conflictingQueries {
				if !c.Expired() {
					allExpired = false
					break
				}
			}

			if allExpired {
				eng.endorse(q, conflictingQueries)
				eng.endorsementMutex.Unlock()
				return
			}
		} else {
			eng.endorsementMutex.Unlock()
			return
		}

		eng.endorsementMutex.Unlock()
		time.Sleep(loopDuration) // TODO smarter wake-up?
	}
}

func (eng *Engine) handleEndorsement(e *Endorsement) {
	// Verify signature
	err := eng.verifyEndorsement(e)
	if err != nil {
		return
	}

	eng.qs.AddEndorsement(e)
	eng.checkState(e.Uuid)
	eng.markActive()
}

func (eng *Engine) handleCheckpoint(ctx context.Context, sc *StartCheckpoint) {
	if len(sc.Queries) == 0 {
		return
	}

	// Compute checkpoint identifier
	sort.Strings(sc.Queries)
	hash := sha256.New()
	for _, uuid := range sc.Queries {
		_, _ = hash.Write([]byte(uuid))
	}

	sum := fmt.Sprintf("%d-%x", len(sc.Queries), hash.Sum(nil))
	_, err := eng.checkpoints.GetIFPresent(sum)

	// TODO check if we need to resend confirmation?
	if err != nil {
		_ = eng.checkpoints.SetWithExpire(sum, true, 60*time.Second)
		choice, proofs := eng.qs.CheckpointChoice(sc.Queries)

		zap.L().Debug("Checkpoint",
			zap.String("id", sum),
			zap.String("state", "start"),
			zap.Bool("choice", choice),
		)

		go func() {
			decision, decisionProofs, _ := eng.BBCEngine.Execute(ctx, sum, choice, proofs)

			zap.L().Debug("Checkpoint",
				zap.String("id", sum),
				zap.String("state", "end"),
				zap.Bool("decision", decision),
			)

			if !decision && choice { // Unexpected veto encountered, process proofs
				for _, proof := range decisionProofs {
					if q := proof.GetQuery(); q != nil {
						eng.handleQuery(q)
					} else if e := proof.GetEndorsement(); e != nil {
						eng.handleEndorsement(e)
					} else {
						zap.L().Warn("Invalid checkpoint proof",
							zap.String("id", sum),
							zap.Any("proof", proof),
						)
					}
				}
			}

			if decision {
				eng.qs.CheckpointDrop(sc.Queries)
				eng.markActive()
			}
		}()
	}
}

func (eng *Engine) checkState(uuid string) {
	commit, checkpoint := eng.qs.CheckState(uuid)
	if commit {
		eng.apply(uuid)
		eng.markActive()
		for _, uuid := range eng.qs.PendingQueries() {
			eng.checkState(uuid)
		}
	}

	if len(checkpoint) > 0 {
		for _, c := range checkpoint {
			select {
			case eng.pendingCheckpoints <- c:
			case <-eng.ctx.Done():
				return
			}
		}
	}
}

func (eng *Engine) canEndorse(q *Query) bool {
	if q.Expired() {
		return false
	}

	eng.Store.Lock()
	defer eng.Store.Unlock()
	for k, v := range q.Requirements {
		_, v2, err := eng.Store.Get(k)
		if err != nil || v2.Matches(v) != nil {
			return false
		}
	}

	// TODO policy compliance
	return true
}

func (eng *Engine) endorse(q *Query, conditions []*Query) {
	cstr := make([]string, len(conditions))
	for i, c := range conditions {
		cstr[i] = c.Uuid
	}

	zap.L().Debug("Endorsed",
		zap.String("uuid", q.Uuid),
		zap.Strings("conditions", cstr),
	)

	e := &Endorsement{
		Uuid:       q.Uuid,
		Emitter:    eng.Identity(),
		Conditions: cstr,
	}
	err := eng.signEndorsement(e)
	if err != nil {
		return
	}

	eng.qs.Endorse(q.Uuid)
	_ = eng.Network.Broadcast(e)
}

func (eng *Engine) apply(uuid string) {
	eng.Store.Lock()
	defer eng.Store.Unlock()

	q := eng.qs.GetQuery(uuid)
	if q == nil {
		return
	}

	values := make(map[string]*operations.Value)
	for _, op := range q.Operations {
		value, ok := values[op.Key]
		if !ok {
			data, v, err := eng.Store.Get(op.Key)
			if err != nil && v != NoVersion {
				return
			}

			values[op.Key] = operations.NewValue(data)
			value = values[op.Key]
		}

		err := op.Exec(value)
		if err != nil {
			return
		}
	}

	keys := make([]string, len(values))
	rawValues := make([][]byte, len(values))
	versions := make([]*Version, len(values))

	var i int
	for k, v := range values {
		keys[i] = k
		rawValues[i] = v.Raw
		versions[i] = NewVersion(v.Raw)
		i++
	}
	_ = eng.Store.SetBatch(keys, rawValues, versions)
}
