/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package operations

// Set sets the output value to the input value raw data.
func Set(input []byte, current *Value) error {
	current.reset()
	current.Raw = input
	return nil
}

// Append appends the raw input to the current value.
func Append(input []byte, current *Value) error {
	current.reset()
	current.Raw = append(current.Raw, input...)
	return nil
}
