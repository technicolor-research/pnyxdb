/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"fmt"
	"time"
)

type cliMap map[string]func(arg string) error

func (c *Client) getCLIMap() cliMap {
	return cliMap{
		"HELP":      c.help,
		"GET":       c.processGET,
		"VERSION":   c.processVERSION,
		"SET":       c.processGeneric2("SET"),
		"CONCAT":    c.processGeneric2("CONCAT"),
		"ADD":       c.processGeneric2("ADD"),
		"MUL":       c.processGeneric2("MUL"),
		"SADD":      c.processGeneric2("SADD"),
		"SREM":      c.processGeneric2("SREM"),
		"SMEMBERS":  c.processMEMBERS,
		"SCONTAINS": c.processCONTAINS,
		"POL":       c.SetPolicy,
		"TIMEOUT":   c.SetTxTimeout,
	}
}

// SetPolicy sets the active client policy. Used for CLI mode mainly.
func (c *Client) SetPolicy(pol string) error {
	c.policy = pol
	return nil
}

// SetTxTimeout sets the default transaction timeout.
func (c *Client) SetTxTimeout(timeout string) error {
	t, err := time.ParseDuration(timeout)
	if err != nil {
		fmt.Println(err)
	} else {
		c.txTimeout = t
	}

	return err
}

func (c *Client) help(string) error {
	fmt.Println("Available commands:")
	for k := range c.climap {
		fmt.Print(k, " ")
	}

	fmt.Println()
	return nil
}
