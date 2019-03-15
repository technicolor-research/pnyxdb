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
	"html/template"
	"os"

	"github.com/spf13/cobra"
)

var configTmpl = template.Must(template.New("config").Parse(
	`# This is a generated configuration file
# Update it to your needs!

identity: {{.ID}}
keyring: {{.Prefix}}{{.ID}}.pem
n: {{.N}}
w: {{.W}}

db:
  path: {{.Prefix}}{{.ID}}.db
  driver: boltdb

p2p:
  listen: "/ip4/0.0.0.0/tcp/4100"
  peers: # uncomment and edit to connect to other peers
    #- "/ip4/172.17.0.1/tcp/4100/p2p/12D3KooWKVwkSqnBQajcAYZNmUrhvDqj59BzBtRzmGd4qYaTv2Y4"
    #- "/ip4/172.17.0.2/tcp/4100/p2p/12D3KooWNaQFB9f1j9MutyoXPuFy3gMA6sxCR2EUUxVg6ShFFaak"

recoveryQuorum: 3

api:
  listen: "127.0.0.1:4200"
`))

// initCmd represents the client command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a simple PnyxDB configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		path := *cfgFile
		if len(path) == 0 {
			path = "config.yaml"
		}

		var t struct {
			ID, Prefix string
			N, W       int
		}

		t.ID = read("Identity", "alice")
		t.Prefix = read("File prefix", "")
		t.N = readInt("Number of participants", 4)
		if t.N < 1 {
			fmt.Fprintln(os.Stderr, "!! The number of participants must be greater than 0")
			os.Exit(1)
		}

		f := readInt("Number of allowed byzantine nodes", (t.N-1)/3)
		t.W = 1 + (t.N+f)/2

		file, err := os.Create(path)
		check(err)
		check(configTmpl.Execute(file, t))
		check(file.Close())

		fmt.Println()
		fmt.Println("Success!")
		fmt.Println("Now you can create your keyring with the following command:")
		fmt.Println()
		fmt.Println(" ", os.Args[0], "-c", path, "keys", "init")
		fmt.Println()
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}
