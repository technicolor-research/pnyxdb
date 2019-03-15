/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/grpc/status"

	"github.com/technicolor-research/pnyxdb/api"
	"github.com/technicolor-research/pnyxdb/consensus"
)

// Get gets the key from the endpoint.
func (c *Client) Get(ctx context.Context, key string) (value []byte, v *consensus.Version, err error) {
	res, err := c.client.Get(ctx, &api.Key{Key: key})
	if res != nil {
		value = res.Data
		v = res.Version
	}

	return
}

// Members returns the slice of every element of a container.
func (c *Client) Members(ctx context.Context, key string) (values [][]byte, v *consensus.Version, err error) {
	members, err := c.client.Members(ctx, &api.Key{Key: key})
	if members != nil {
		values = members.Data
		v = members.Version
	}

	return
}

// Contains returns wether or not a specific value is present in a container.
func (c *Client) Contains(ctx context.Context, key string, value []byte) (contains bool, err error) {
	boolean, err := c.client.Contains(ctx, &api.KeyValue{Key: key, Value: value})
	contains = boolean.Boolean
	return
}

func (c *Client) processGET(arg string) error {
	ctx, done := c.ctx()
	defer done()

	value, _, err := c.Get(ctx, arg)
	if err != nil {
		fmt.Println("Error:", status.Convert(err).Message())
		return err
	}

	fmt.Printf("%s\n", value)
	return nil
}

func (c *Client) processVERSION(arg string) error {
	ctx, done := c.ctx()
	defer done()
	_, v, err := c.Get(ctx, arg)
	if err != nil || v.Matches(consensus.NoVersion) == nil {
		fmt.Println("0x0")
		return err
	}

	fmt.Printf("0x%x\n", v.Hash)
	return nil
}

func (c *Client) processMEMBERS(arg string) error {
	ctx, done := c.ctx()
	defer done()
	values, _, err := c.Members(ctx, arg)
	if err != nil {
		fmt.Println("Error:", status.Convert(err).Message())
		return err
	}

	fmt.Println(len(values), "element(s)")

	strValues := make([]string, len(values))
	for i, data := range values {
		strValues[i] = string(data)
	}

	sort.Strings(strValues)

	for _, data := range strValues {
		fmt.Printf("- %s\n", data)
	}

	return nil
}

func (c *Client) processCONTAINS(arg string) error {
	ctx, done := c.ctx()
	defer done()
	arg1, arg2, err := split2args(arg)
	if err != nil {
		fmt.Println("CONTAINS function expects two arguments: (container, element)")
		return err
	}

	contains, err := c.Contains(ctx, arg1, []byte(arg2))
	if err != nil {
		fmt.Println("Error:", status.Convert(err).Message())
	}

	fmt.Println(contains)
	return nil
}
