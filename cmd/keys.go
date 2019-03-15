/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/awnumar/memguard"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/technicolor-research/pnyxdb/keyring"
)

const strTrustLevel = "none,low,high,ultimate"

var errMissingIdentity = errors.New("missing 'identity' key from configuration file")

func getSelfIdentity() string {
	identity := viper.GetString("identity")
	if identity == "" {
		check(errMissingIdentity)
	}

	return identity
}

func getPassword() *memguard.LockedBuffer {
	password := viper.GetString("password")
	if len(password) == 0 {
		check(errors.New("please provide a password through `PASSWORD` environment variable"))
	}

	buffer, err := memguard.NewImmutableFromBytes([]byte(password))
	check(err)

	viper.Set("password", nil)
	return buffer
}

func getKeyRing() *keyring.KeyRing {
	rawKeyRing, err := ioutil.ReadFile(viper.GetString("keyring"))
	check(err)

	keyRing, err := keyring.NewKeyRing(getSelfIdentity(), "ed25519")
	check(err)
	check(keyRing.UnmarshalBinary(rawKeyRing))
	return keyRing
}

func saveKeyRing(keyRing *keyring.KeyRing) {
	data, err := keyRing.MarshalBinary()
	check(err)
	check(ioutil.WriteFile(viper.GetString("keyring"), data, 0600))
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage signature keys",
}

var keysInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create local keyring",
	Run: func(cmd *cobra.Command, args []string) {
		check(cfgErr)
		// Generate new KeyRing
		keyRing, err := keyring.NewKeyRing(getSelfIdentity(), "ed25519")
		check(err)
		check(keyRing.CreatePrivate(getPassword()))

		// Save to disk
		saveKeyRing(keyRing)

		// Print confirmation
		pub, _, _ := keyRing.GetPublic(keyRing.Identity())
		fmt.Printf("Generated new keyring (%s)\n", keyring.Fingerprint(pub))
	},
}

var keysExportCmd = &cobra.Command{
	Use:   "export [identity]",
	Short: "Export a public key from the keyring",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()

		if len(args) == 0 {
			args = []string{keyRing.Identity()}
		}

		data, err := keyRing.Export(args[0])
		check(err)
		fmt.Printf("%s", data)
	},
}

var importTrust *string

var keysImportCmd = &cobra.Command{
	Use:   "import [id]",
	Short: "Import a public key to the keyring",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()
		identity := getIdentity(cmd, args)

		lvl, err := keyring.ParseTrust(*importTrust)
		check(err)

		data, err := ioutil.ReadAll(os.Stdin)
		check(err)
		check(keyRing.Import(data, identity, lvl))

		saveKeyRing(keyRing)

		pub, _, _ := keyRing.GetPublic(identity)
		fmt.Printf("Imported new key for identity %s (%s) with %s trust level\n", args[0], keyring.Fingerprint(pub), lvl)
	},
}

var keysRemoveCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Remove a public key from the keyring",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()
		identity := getIdentity(cmd, args)

		keyRing.RemovePublic(identity)
		saveKeyRing(keyRing)
	},
}

var keysListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List public keys from the keyring",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Identity", "Trust", "Certified", "Fingerprint"})
		table.SetRowLine(true)
		table.SetAutoFormatHeaders(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, k := range keyRing.ListPublic() {
			identity, data, trust := k.Info()

			cert := "✔️️  yes"
			if keyRing.Trusted(identity) != nil {
				cert = "❌ no"
			}

			if identity == keyRing.Identity() {
				identity = "<self>"
			}

			table.Append([]string{identity, trust.String(), cert, keyring.Fingerprint(data)})
		}

		table.Render()
	},
}

var keysShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Get informations about a specific identity",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()
		identity := getIdentity(cmd, args)

		data, trust, err := keyRing.GetPublic(identity)
		check(err)

		signatures := keyRing.GetSignatures(identity)

		status := "Certified"
		err = keyRing.Trusted(identity)
		if err != nil {
			status = fmt.Sprintf("Insufficient trust (%d/%d)", err.(*keyring.ErrInsufficientTrust).L, keyring.TrustThreshold)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetColWidth(100)
		table.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})
		table.SetAutoMergeCells(true)
		table.SetColumnSeparator(":")

		table.Append([]string{"Identity", identity})
		table.Append([]string{"Trust", trust.String()})
		table.Append([]string{"Fingerprint", keyring.Fingerprint(data)})
		table.Append([]string{"Public key", fmt.Sprintf("%X", data)})
		table.Append([]string{"Status", status})

		for i, s := range signatures {
			if i == keyRing.Identity() {
				i = "<self>"
			}
			table.Append([]string{"Approved by", fmt.Sprintf("%s (%s)", i, s.Trust)})
		}

		if len(signatures) == 0 {
			table.Append([]string{"Approved by", "(nobody)"})
		}

		table.Render()
	},
}

var keysTrustCmd = &cobra.Command{
	Use:   "trust [id] [" + strTrustLevel + "]",
	Short: "Update local trust level in specific key",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()
		identity := getIdentity(cmd, args)
		lvl, err := keyring.ParseTrust(getArg(cmd, args, 1))
		check(err)

		data, _, err := keyRing.GetPublic(identity)
		check(err)
		check(keyRing.AddPublic(identity, lvl, data))
		saveKeyRing(keyRing)
	},
}

var keysSignCmd = &cobra.Command{
	Use:   "sign [id]",
	Short: "Sign an identity with private key according to stored trust level",
	Run: func(cmd *cobra.Command, args []string) {
		keyRing := getKeyRing()
		password := getPassword()
		identity := getIdentity(cmd, args)
		check(keyRing.UnlockPrivate(password))
		check(keyRing.AddSignature(identity, keyRing.Identity(), nil))
		saveKeyRing(keyRing)
	},
}

func getIdentity(cmd *cobra.Command, args []string) string {
	return getArg(cmd, args, 0)
}

func init() {
	keysCmd.AddCommand(
		keysInitCmd,
		keysExportCmd,
		keysImportCmd,
		keysRemoveCmd,
		keysListCmd,
		keysShowCmd,
		keysTrustCmd,
		keysSignCmd,
	)
	RootCmd.AddCommand(keysCmd)

	importTrust = keysImportCmd.Flags().StringP("trust", "t", "low", "public key local trust ("+strTrustLevel+")")
}
