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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	uuid "github.com/satori/go.uuid"
)

// NewQuery instanciates a new empty query.
func NewQuery() *Query {
	s := &Query{}
	u, _ := uuid.NewV4()
	s.Uuid = u.String()
	s.Policy = "none"
	s.Requirements = make(map[string]*Version)
	return s
}

// CheckConflict returns an error if two queries are conflicting.
func (q *Query) CheckConflict(q2 *Query) error {
	if q == nil || q2 == nil {
		return nil
	}

	if q.Policy != q2.Policy {
		return nil
	}

	for _, op := range q.Operations {
		for _, op2 := range q2.Operations {
			if err := op.CheckConflict(op2); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetTimeout returns the duration that is remaining for the application of this query.
// This may be negative.
func (q *Query) GetTimeout() time.Duration {
	if q == nil {
		return 0
	}

	t := time.Unix(q.Deadline.Seconds, int64(q.Deadline.Nanos))
	return t.Sub(time.Now())
}

// SetTimeout updates the deadline of the query according to current time.
func (q *Query) SetTimeout(t time.Duration) {
	if q == nil {
		return
	}

	deadline := time.Now().Add(t)

	var err error
	q.Deadline, err = ptypes.TimestampProto(deadline)
	if err != nil { // Gracefully handle error
		q.Deadline = ptypes.TimestampNow()
	}
}

// DeadlineTime returns the query deadline in native time  instead of ptype.
func (q *Query) DeadlineTime() time.Time {
	// Error ignored according to https://github.com/golang/protobuf/blob/master/ptypes/timestamp.go
	// "Every valid Timestamp can be represented by a time.Time, but the converse is not true."
	t, _ := ptypes.Timestamp(q.Deadline)
	return t
}

// Expired returns true if a query deadline is reached.
func (q *Query) Expired() bool {
	return q.ExpiredSince(0)
}

// ExpiredSince returns true if a query deadline have been reached for at least d duration.
func (q *Query) ExpiredSince(d time.Duration) bool {
	if q == nil || q.Deadline == nil {
		return true
	}

	limit := time.Now().Add(-1 * d)
	return !q.DeadlineTime().After(limit)
}

// Hash returns a fixed-size hash of the (unsigned) version of the query.
// Passed by value because of internal modifications.
func (q Query) Hash() ([]byte, error) {
	q.Signature = nil
	raw, err := proto.Marshal(&q)
	hash := sha512.Sum512(raw)
	return hash[:], err
}
