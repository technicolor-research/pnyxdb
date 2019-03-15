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
	"io"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/technicolor-research/pnyxdb/api"
	"github.com/technicolor-research/pnyxdb/consensus"
	"google.golang.org/grpc/status"
)

// Submit submits the transaction to the endpoint.
func (c *Client) Submit(ctx context.Context, tx *api.Transaction) (uuid string, err error) {
	res, err := c.client.Submit(ctx, tx)
	if err != nil {
		return
	}

	uuid = res.Uuid
	return
}

func (c *Client) processGeneric2(op string) func(arg string) error {
	return func(arg string) error {
		arg1, arg2, err := split2args(arg)
		if err != nil {
			fmt.Println(op, "function expects two arguments: (key, data)")
			return err
		}

		timeout := c.txTimeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		deadline, _ := ptypes.TimestampProto(time.Now().Add(timeout))

		tx := &api.Transaction{
			Operations: []*consensus.Operation{{
				Key:  arg1,
				Op:   consensus.Operation_Op(consensus.Operation_Op_value[op]),
				Data: []byte(arg2),
			}},
			Policy:   c.policy,
			Deadline: deadline,
		}

		ctx, done := c.ctx()
		defer done()

		uuid, err := c.Submit(ctx, tx)
		if err != nil {
			fmt.Println("Error:", status.Convert(err).Message())
			return err
		}

		fmt.Println(uuid)
		return nil
	}
}

func split2args(arg string) (arg1, arg2 string, err error) {
	args := strings.SplitN(arg, " ", 2)
	if len(args) < 2 {
		return "", "", io.ErrUnexpectedEOF
	}

	return args[0], args[1], nil
}
