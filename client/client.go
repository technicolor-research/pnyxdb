/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package client provides Consensus API client.
//
// It can be used by external applications willing to communicate with a single node.
// The API client is able to get current state of one node's database, and submit transactions
// to the whole cluster.
package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/technicolor-research/pnyxdb/api"

	"github.com/chzyer/readline"
	"google.golang.org/grpc"
)

// Client is the GRPC PnyxDB client.
type Client struct {
	Addr    string
	Timeout time.Duration

	conn      *grpc.ClientConn
	client    api.EndorserClient
	policy    string
	txTimeout time.Duration
	climap    cliMap
}

// Connect proceeds to the GRPC connection step to the server.
func (c *Client) Connect() (err error) {
	ctx, cancel := context.WithTimeout(context.TODO(), c.Timeout)
	defer cancel()

	c.conn, err = grpc.DialContext(ctx, c.Addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}

	c.client = api.NewEndorserClient(c.conn)
	c.climap = c.getCLIMap()
	return nil
}

// Close closes the GRPC connection to the server.
func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// CLI starts a command line interface to dial with the GRPC server (debug and maintenance).
func (c *Client) CLI() {
	rl, err := readline.New(c.Addr + "> ")
	if err != nil {
		return
	}
	defer func() { _ = rl.Close() }()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		_ = c.Run(line)
		time.Sleep(100 * time.Millisecond)
	}
}

// Run runs a single expression against the client.
// An expression is expected to be a command followed by a number of arguments.
func (c *Client) Run(expression string) error {
	args := strings.SplitN(expression, " ", 2)
	cmd := strings.ToUpper(args[0])

	f, ok := c.climap[cmd]
	if !ok {
		fmt.Println("Invalid command")
		err := errors.New("invalid command")
		return err
	}

	arg := ""
	if len(args) > 1 {
		arg = args[1]
	}
	return f(arg)
}

func (c *Client) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.Timeout)
}
