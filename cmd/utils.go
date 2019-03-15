/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/awnumar/memguard"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		memguard.SafeExit(1)
	}
}

func getArg(cmd *cobra.Command, args []string, index int) string {
	if len(args) <= index || args[index] == "" {
		_ = cmd.Usage()
		os.Exit(1)
	}

	return args[index]
}

func read(s string, d string) string {
	l, err := readline.Line(s + " [" + d + "]: ")
	check(err)

	if l == "" {
		l = d
	}

	return l
}

func readInt(s string, d int) int {
	for {
		l := read(s, strconv.Itoa(d))
		n, err := strconv.Atoi(l)
		if err == nil {
			return n
		}
	}
}
