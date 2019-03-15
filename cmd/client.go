/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/technicolor-research/pnyxdb/client"
)

var addrSrv *string
var timeoutSrv *time.Duration
var policy *string
var txTimeout *time.Duration

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client [command]",
	Short: "Run a PnyxDB client in CLI",
	Run: func(cmd *cobra.Command, args []string) {
		cli := &client.Client{
			Addr:    *addrSrv,
			Timeout: *timeoutSrv,
		}

		err := cli.Connect()
		check(err)

		_ = cli.SetPolicy(*policy)
		_ = cli.SetTxTimeout(txTimeout.String())

		var status int
		if len(args) == 0 {
			cli.CLI()
		} else {
			err = cli.Run(strings.Join(args, " "))
			if err != nil {
				status = 1
			}
		}
		cli.Close()
		os.Exit(status)
	},
}

func init() {
	RootCmd.AddCommand(clientCmd)
	addrSrv = clientCmd.Flags().StringP("server", "s", "localhost:4200", "server address")
	timeoutSrv = clientCmd.Flags().DurationP("timeout", "t", 10*time.Second, "connection timeout")
	policy = clientCmd.Flags().StringP("policy", "p", "none", "default policy to use when submitting")
	txTimeout = clientCmd.Flags().DurationP("txtimeout", "x", 5*time.Second, "transaction timeout")
}
