/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package operations

import "github.com/technicolor-research/pnyxdb/consensus/encoding"

// Value holds the data that shall be used by operations.
// One value, and only one, shall be used per key on a given transaction.
// See the Runner interface for an example of usage.
type Value struct {
	Raw []byte

	vfloat *encoding.Float
	vset   *encoding.Set
}

// NewValue returns a new value.
func NewValue(raw []byte) *Value {
	return &Value{Raw: raw}
}

func (v *Value) reset() {
	v.vfloat = nil
	v.vset = nil
}

// Float lazily returns the current float value.
func (v *Value) Float() (*encoding.Float, error) {
	if v.vfloat != nil {
		return v.vfloat, nil
	}

	vfloat := encoding.NewFloat()
	err := vfloat.UnmarshalBinary(v.Raw)
	if err != nil {
		return nil, err
	}

	v.vfloat = vfloat
	return vfloat, nil
}

// Set lazily returns the current set value.
func (v *Value) Set() (*encoding.Set, error) {
	if v.vset != nil {
		return v.vset, nil
	}

	vset := encoding.NewSet()
	err := vset.UnmarshalBinary(v.Raw)
	if err != nil {
		return nil, err
	}

	v.vset = vset
	return vset, nil
}
