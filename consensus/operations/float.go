/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package operations

import "github.com/technicolor-research/pnyxdb/consensus/encoding"

func floatGeneric(input []byte, current *Value, add bool) error {
	a := encoding.NewFloat()
	b, err := current.Float()
	err2 := a.UnmarshalBinary(input)
	if err != nil || err2 != nil {
		return ErrNotNumeric
	}

	if add {
		current.vfloat = a.Add(b)
	} else {
		current.vfloat = a.Mul(b)
	}

	current.Raw, err = current.vfloat.MarshalText()
	return err
}

// Add adds the input as float to the current value.
func Add(input []byte, current *Value) error {
	return floatGeneric(input, current, true)
}

// Mul multiplies the input as float to the current value.
func Mul(input []byte, current *Value) error {
	return floatGeneric(input, current, false)
}
