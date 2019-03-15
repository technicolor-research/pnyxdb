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
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryStore_AddEndorsement(t *testing.T) {
	qs := newQueryStore()
	q := NewQuery()

	require.True(t, qs.AddQuery(q))

	testCases := []struct {
		emitter, uuid     string
		pending, inserted bool
	}{
		{"a", q.Uuid, false, true},
		{"b", q.Uuid, false, true},
		{"a", q.Uuid, false, false},
		{"a", "unknown", true, false},
	}

	for _, tc := range testCases {
		c := tc
		t.Run(fmt.Sprintf("%s/%s", c.emitter, c.uuid), func(t *testing.T) {
			e := &Endorsement{Emitter: c.emitter, Uuid: c.uuid}
			pending, inserted := qs.AddEndorsement(e)
			require.Equal(t, c.pending, pending)
			require.Equal(t, c.inserted, inserted)
		})
	}
}

func TestQueryStore_isApplicable(t *testing.T) {
	// From the original paper (figure 1)
	q := NewQuery()
	r := NewQuery()

	eq1 := &Endorsement{Emitter: "1", Uuid: q.Uuid}
	eq2 := &Endorsement{Emitter: "2", Uuid: q.Uuid}
	eq3 := &Endorsement{Emitter: "3", Uuid: q.Uuid}
	er1 := &Endorsement{Emitter: "1", Uuid: r.Uuid, Conditions: []string{q.Uuid}}
	er2 := &Endorsement{Emitter: "2", Uuid: r.Uuid, Conditions: []string{q.Uuid}}
	er4 := &Endorsement{Emitter: "4", Uuid: r.Uuid}

	t.Run("Simple", func(t *testing.T) {
		qs := newQueryStore()
		qs.threshold = 3

		qs.AddQuery(q)
		qs.AddQuery(r)
		qs.AddEndorsement(eq1)
		qs.AddEndorsement(eq2)
		qs.AddEndorsement(er1)
		qs.AddEndorsement(er2)
		qs.AddEndorsement(er4)

		require.True(t, qs.isApplicable(r.Uuid), "r has reached 3 valid endorsements, must be applicable")
		require.False(t, qs.isApplicable(q.Uuid), "q has only reached 2 valid endorsements, must NOT be applicable")

		qs.AddEndorsement(eq3)
		require.False(t, qs.isApplicable(r.Uuid), "r has now only 1 valid endorsement, must NOT be applicable")
		require.True(t, qs.isApplicable(q.Uuid), "q has now reached 3 valid endorsements, must be applicable")
	})

	t.Run("OutOfOrder", func(t *testing.T) {
		qs := newQueryStore()
		qs.threshold = 3

		qs.AddEndorsement(eq1)
		qs.AddEndorsement(er1)
		qs.AddQuery(r)
		qs.AddEndorsement(er2)
		qs.AddEndorsement(er4)
		qs.AddEndorsement(eq2)

		require.True(t, qs.isApplicable(r.Uuid), "r has reached 3 valid endorsements, must be applicable")

		qs.AddEndorsement(eq3)
		qs.AddQuery(q)
		require.False(t, qs.isApplicable(r.Uuid), "r has now only 1 valid endorsement, must NOT be applicable")
		require.True(t, qs.isApplicable(q.Uuid), "q has now reached 3 valid endorsements, must be applicable")
	})

	t.Run("Parallel", func(t *testing.T) {
		n := 100 // Number of queries
		m := 50  // Number of endorsements per query

		qs := newQueryStore()
		qs.threshold = m

		var wg sync.WaitGroup
		wg.Add(n*m + n)

		// Build queries
		var queries []*Query
		buildCache := func() {
			qs.Lock()
			defer qs.Unlock()
			for _, q := range queries {
				qs.isApplicable(q.Uuid)
			}
		}

		for i := 0; i < n; i++ {
			queries = append(queries, NewQuery())
		}
		for i := 0; i < n; i++ {
			go func(i int) {
				qs.AddQuery(queries[i])
				buildCache()
				wg.Done()
			}(i)
		}

		// Generate endorsements
		for i := 0; i < n*m; i++ {
			var conditions []string
			for j := 0; j < i/m; j++ {
				conditions = append(conditions, queries[j].Uuid)
			}

			endorsement := &Endorsement{
				Emitter:    strconv.Itoa(i % m),
				Uuid:       queries[i/m].Uuid,
				Conditions: conditions,
			}

			go func() {
				qs.AddEndorsement(endorsement)
				buildCache()
				wg.Done()
			}()
		}

		wg.Wait()

		for i, q := range queries {
			if i == 0 {
				require.True(t, qs.isApplicable(q.Uuid), "first query must be applicable")
			} else {
				require.False(t, qs.isApplicable(q.Uuid), fmt.Sprintf("#%d query must NOT be applicable", i))
			}
		}

		t.Run("DumpLoad", func(t *testing.T) {
			buffer := &bytes.Buffer{}
			qs2 := newQueryStore()
			require.Nil(t, qs.Dump(buffer), "should be able to dump large query store")
			require.Nil(t, qs2.Load(buffer), "should be able to load large query store")
			require.Equal(t, len(qs.queries), len(qs2.queries))
			require.Equal(t, len(qs.pendingDependencies), len(qs2.pendingDependencies))
			require.Equal(t, len(qs.pendingEndorsements), len(qs2.pendingEndorsements))
		})

		t.Run("Drop", func(t *testing.T) {
			qs.drop(queries[0].Uuid)

			for i, q := range queries {
				if i == 1 {
					require.True(t, qs.isApplicable(q.Uuid), "second query must be applicable")
				} else {
					require.False(t, qs.isApplicable(q.Uuid), fmt.Sprintf("#%d query must NOT be applicable", i))
				}
			}
		})
	})
}

func BenchmarkQueryStore_AddEndorsement(b *testing.B) {
	qs := newQueryStore()
	q := NewQuery()
	qs.AddQuery(q)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qs.AddEndorsement(&Endorsement{Emitter: strconv.Itoa(i), Uuid: q.Uuid})
	}
}
