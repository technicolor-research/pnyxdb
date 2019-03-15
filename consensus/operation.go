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
	"errors"

	"github.com/technicolor-research/pnyxdb/consensus/operations"
)

// ParallelType specifies the various options available when specifying a parallelizable operation.
type ParallelType int16

// Definition for ParallelType.
// Each flag may be combined using bitwise operators.
const (
	ParallelTypeDEFAULT ParallelType = 0x01 << iota
	ParallelTypeDISALLOWDIFFERENT
	ParallelTypeDISALLOWEQUAL
)

// ParallelMatrix is used to know which operation can be run in parallel on a specific object.
var ParallelMatrix = map[Operation_Op]map[Operation_Op]ParallelType{
	Operation_SET: {Operation_SET: ParallelTypeDISALLOWDIFFERENT},
	Operation_ADD: {Operation_ADD: ParallelTypeDEFAULT},
	Operation_MUL: {Operation_MUL: ParallelTypeDEFAULT},
	Operation_SADD: {
		Operation_SADD: ParallelTypeDEFAULT,
		Operation_SREM: ParallelTypeDISALLOWEQUAL,
	},
	Operation_SREM: {
		Operation_SREM: ParallelTypeDEFAULT,
		Operation_SADD: ParallelTypeDISALLOWEQUAL,
	},
}

var runners = map[Operation_Op]operations.Runner{
	Operation_SET:    operations.Set,
	Operation_CONCAT: operations.Append,
	Operation_ADD:    operations.Add,
	Operation_MUL:    operations.Mul,
	Operation_SADD:   operations.Sadd,
	Operation_SREM:   operations.Srem,
}

// CheckConflict returns an error if two operations cannot be executed in parallel.
func (o *Operation) CheckConflict(o2 *Operation) error {
	err := errors.New("non-parallel operations " + o.Op.String() + " / " + o2.Op.String())
	if o.Key != o2.Key {
		return nil
	}

	if ParallelMatrix[o.Op] == nil {
		return err
	}

	t := ParallelMatrix[o.Op][o2.Op]
	if t == 0 {
		return err
	}

	if ParallelTypeDEFAULT&t > 0 {
		return nil // bypass further checks
	}

	equal := bytes.Equal(o.Data, o2.Data)
	if equal && ParallelTypeDISALLOWEQUAL&t > 0 {
		return err
	}

	if !equal && ParallelTypeDISALLOWDIFFERENT&t > 0 {
		return err
	}

	return nil
}

// Exec returns the result of the given operation against stored data.
func (o *Operation) Exec(v *operations.Value) error {
	r, implemented := runners[o.Op]
	if !implemented {
		return errors.New("operation not yet implemented")
	}

	return r(o.Data, v)
}
