/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

const deltaOld = 1 * time.Second

type queryState byte

const (
	qPending queryState = iota
	qCommitted
	qDropped
)

type cachedInfo struct {
	result, fresh bool
}

func (ci cachedInfo) Fresh() bool { return ci.fresh }
func (ci cachedInfo) Get() bool   { return ci.result }
func (ci *cachedInfo) Set(r bool) { ci.result, ci.fresh = r, true }
func (ci *cachedInfo) Mark()      { ci.fresh = false }

type queryInfo struct {
	*Query

	// note: we store endorsement and query information as value for GC simplification,
	//       beware of copied-values when updating state
	Endorsements []endorsementInfo
	Dependents   []string
	State        queryState
	Endorsed     bool
	Applied      bool
	cachedInfo
}

type endorsementInfo struct {
	*Endorsement
	cachedInfo
}

type queryStore struct {
	sync.RWMutex

	queries             map[string]queryInfo
	pendingDependencies map[string][]string
	pendingEndorsements []*Endorsement
	threshold           int
}

func newQueryStore() *queryStore {
	return &queryStore{
		queries:             make(map[string]queryInfo),
		pendingDependencies: make(map[string][]string),
	}
}

func (qs *queryStore) AddQuery(q *Query) (inserted bool) {
	qs.Lock()
	defer qs.Unlock()

	if _, ok := qs.queries[q.Uuid]; ok {
		return
	}

	qi := queryInfo{Query: q}

	// Trick from https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	pendingEndorsements := qs.pendingEndorsements[:0]
	defer func() { qs.pendingEndorsements = pendingEndorsements }()

	for _, pe := range qs.pendingEndorsements {
		if pe.Uuid != q.Uuid {
			pendingEndorsements = append(pendingEndorsements, pe)
			continue
		}
		_, qi = qs.addEndorsementInternal(pe, qi)
	}

	qi.Dependents = qs.pendingDependencies[q.Uuid]
	delete(qs.pendingDependencies, q.Uuid)

	inserted = true
	qi.Set(false) // force marking cascade by setting a default value
	qs.cascadeMark(qi)
	return
}

func (qs *queryStore) GetQuery(uuid string) *Query {
	qs.RLock()
	defer qs.RUnlock()

	qi := qs.queries[uuid]
	return qi.Query
}

func (qs *queryStore) AddEndorsement(e *Endorsement) (pending bool, inserted bool) {
	qs.Lock()
	defer qs.Unlock()

	qi, ok := qs.queries[e.Uuid]
	if !ok {
		qs.pendingEndorsements = append(qs.pendingEndorsements, e)
		pending = true
		return
	}

	inserted, qi = qs.addEndorsementInternal(e, qi)
	qs.cascadeMark(qi)
	return
}

func (qs *queryStore) addEndorsementInternal(e *Endorsement, qi queryInfo) (bool, queryInfo) { // unsafe
	// Is there already an endorsement from the emitter?
	for _, e2 := range qi.Endorsements {
		if e.Emitter == e2.Emitter {
			return false, qi
		}
	}

	for _, c := range e.Conditions {
		qic, ok := qs.queries[c]
		if ok {
			qic.Dependents = addToSet(qic.Dependents, qi.Uuid)
			qs.queries[c] = qic
		} else {
			qs.pendingDependencies[c] = addToSet(qs.pendingDependencies[c], qi.Uuid)
		}
	}

	qi.Endorsements = append(qi.Endorsements, endorsementInfo{Endorsement: e})
	return true, qi
}

func (qs *queryStore) cascadeMark(qi queryInfo) { // unsafe
	if qi.Query == nil {
		fmt.Println("!!!!!")
		return
	}

	marked := !qi.Fresh()
	qi.Mark()
	qs.queries[qi.Uuid] = qi

	// Do not propagate if already marked
	if marked {
		return
	}

	for _, uuid := range qi.Dependents {
		qid, ok := qs.queries[uuid]
		if !ok {
			continue
		}

		for i, e := range qid.Endorsements {
			var contains bool
			for _, c := range e.Conditions {
				if c == qi.Uuid {
					contains = true
					break
				}
			}

			if contains {
				e.Mark()
				qid.Endorsements[i] = e
			}
		}

		qs.cascadeMark(qid)
	}
}

func (qs *queryStore) isApplicable(uuid string) bool { // unsafe
	q, ok := qs.queries[uuid]
	if !ok || q.State == qDropped {
		return false
	}

	if q.State == qCommitted {
		return true
	}

	if q.Fresh() {
		return q.Get()
	}

	var result bool
	defer func() {
		q.Set(result)
		qs.queries[uuid] = q
	}()

	// Optimize if the number of received endorsements is not high enough
	if len(q.Endorsements) < qs.threshold { // TODO per-policy threshold
		return result
	}

	var valid int
	for i, e := range q.Endorsements {
		if !e.Fresh() {
			ok := true
			for _, c := range e.Conditions {
				if qs.isApplicable(c) {
					ok = false
					break
				}
			}

			e.Set(ok)
			q.Endorsements[i] = e
		}

		if e.Get() {
			valid++
		}
	}

	result = valid >= qs.threshold
	return result
}

func (qs *queryStore) GetConflicting(q *Query) (cq []*Query) {
	if q == nil || qs == nil {
		return
	}

	qs.RLock()
	defer qs.RUnlock()

	for uuid, q2 := range qs.queries {
		if uuid == q.Uuid {
			continue // same query
		}

		if q2.State != qPending {
			continue // query has already been processed
		}

		if !q2.Endorsed {
			continue // query has not been endorsed locally
		}

		if q.CheckConflict(q2.Query) != nil {
			cq = append(cq, q2.Query)
		}
	}

	return
}

func (qs *queryStore) CheckState(uuid string) (commit bool, checkpoint []string) {
	qs.Lock()
	defer qs.Unlock()

	applicable := qs.isApplicable(uuid)
	qs.checkSpeculativeState(uuid, applicable)
	if !applicable {
		return
	}

	if qs.queries[uuid].State == qCommitted {
		return
	}

	n := 0
	for _, e := range qs.queries[uuid].Endorsements {
		definitelyValid := true
		for _, c := range e.Conditions {
			qi, ok := qs.queries[c]
			if !ok || qi.State != qDropped {
				definitelyValid = false

				old := !ok || !qs.isApplicable(c) && qi.ExpiredSince(deltaOld)
				if old {
					checkpoint = addToSet(checkpoint, c)
				}

				break
			}
		}

		if definitelyValid {
			n++
		}
	}

	if n >= qs.threshold { // TODO per policy threshold
		commit = true
		qs.commit(uuid)
	}

	return commit, checkpoint
}

func (qs *queryStore) PendingQueries() []string {
	qs.RLock()
	defer qs.RUnlock()

	var out []string
	for _, qi := range qs.queries {
		if qi.State == qPending {
			out = append(out, qi.Uuid)
		}
	}

	return out
}

func (qs *queryStore) OutdatedQueries() []string {
	qs.Lock()
	defer qs.Unlock()

	var out []string
	for _, qi := range qs.queries {
		if qi.State == qPending && !qs.isApplicable(qi.Uuid) && qi.GetTimeout() < -10*time.Second {
			out = append(out, qi.Uuid)
		}
	}

	return out

}

func (qs *queryStore) CheckpointChoice(queries []string) (choice bool, proofs []*Proof) {
	qs.Lock()
	defer qs.Unlock()

	for _, uuid := range queries {
		if qs.isApplicable(uuid) {
			zap.L().Debug("Veto",
				zap.String("uuid", uuid),
				zap.String("reason", "applicable"),
			)

			qi, _ := qs.queries[uuid]
			proofs = []*Proof{{Content: &Proof_Query{qi.Query}}}
			for _, ei := range qi.Endorsements {
				proofs = append(proofs, &Proof{
					Content: &Proof_Endorsement{ei.Endorsement},
				})
			}

			return false, proofs
		}
	}

	return true, nil
}

func (qs *queryStore) CheckpointDrop(queries []string) {
	qs.Lock()
	defer qs.Unlock()

	for _, uuid := range queries {
		qs.drop(uuid)
	}
}

func (qs *queryStore) Endorse(uuid string) {
	qs.Lock()
	defer qs.Unlock()

	qi, ok := qs.queries[uuid]
	if !ok {
		return
	}

	qi.Endorsed = true
	qs.queries[uuid] = qi
}

func (qs *queryStore) drop(uuid string) { // unsafe
	qi, ok := qs.queries[uuid]
	if !ok {
		qi = queryInfo{}
	}

	qi.State = qDropped
	qi.Set(false)
	qs.cascadeMark(qi)

	zap.L().Debug("Dropped",
		zap.String("uuid", uuid),
	)
}

func (qs *queryStore) commit(uuid string) { // unsafe
	qi, ok := qs.queries[uuid]
	if !ok {
		qi = queryInfo{}
	}

	qi.State = qCommitted
	qs.queries[uuid] = qi

	// Drop dependents synchronously
	for _, dep := range qi.Dependents {
		qs.drop(dep)
	}

	zap.L().Debug("Committed",
		zap.String("uuid", uuid),
	)
}

// TODO add (real) speculative execution
func (qs *queryStore) checkSpeculativeState(uuid string, applicable bool) {
	qi, ok := qs.queries[uuid]
	if !ok {
		return
	}

	if !applicable && qi.Applied {
		zap.L().Debug("Rollbacked",
			zap.String("uuid", uuid),
		)
		qi.Applied = false
	}

	if applicable && !qi.Applied {
		zap.L().Debug("Applied",
			zap.String("uuid", uuid),
		)
		qi.Applied = true
	}

	qs.queries[uuid] = qi
}

func addToSet(set []string, value string) []string {
	for _, v := range set {
		if value == v {
			return set
		}
	}

	return append(set, value)
}
