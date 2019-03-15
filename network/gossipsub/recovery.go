/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package gossipsub

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"

	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/network/protocol"
	"go.uber.org/zap"
)

const recoveryProtocolID = "/p2p/pnyxdb_recovery"

func (n *network) RequestRecovery(ctx context.Context, key string) (*consensus.RecoveryResponse, error) {
	if n == nil || n.RecoveryQuorum == 0 {
		return nil, nil
	}

	peers := n.ListPeers(n.Topic)
	if uint(len(peers)) < n.RecoveryQuorum {
		return nil, fmt.Errorf("not enough peers to recover, got %d but expected %d", len(peers), n.RecoveryQuorum)
	}

	perm := n.rand.Perm(len(peers))
	req, err := protocol.Pack(&consensus.RecoveryRequest{Key: key})
	if err != nil {
		return nil, err
	}

	zap.L().Info("StartRecovery",
		zap.String("key", key),
		zap.Uint("quorum", n.RecoveryQuorum),
	)

	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resChan := make(chan *consensus.RecoveryResponse, 0)
	for i := 0; uint(i) < n.RecoveryQuorum; i++ {
		go func(i int) {
			select {
			case resChan <- n.recoveryStream(subctx, req, peers[perm[i]]):
			case <-ctx.Done():
			}
		}(i)
	}

	var responses []*consensus.RecoveryResponse

	for {
		select {
		case res := <-resChan:
			if res == nil {
				return nil, errors.New("invalid response from one peer")
			}

			responses = append(responses, res)
			if uint(len(responses)) == n.RecoveryQuorum {
				return n.checkRecoveryResponses(key, responses)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (n *network) checkRecoveryResponses(
	key string, responses []*consensus.RecoveryResponse) (*consensus.RecoveryResponse, error) {
	// we just need to check that:
	// * every response contains the right key
	// * every version is the same
	// * every data is the same

	if len(responses) == 0 {
		return nil, errors.New("undefined behavior")
	}

	refVersion := responses[0].GetVersion()
	refData := responses[0].GetData()

	for i := 0; i < len(responses); i++ {
		if responses[0].GetKey() != key {
			return nil, errors.New("key mismatch")
		}

		if i > 0 {
			if refVersion.Matches(responses[i].GetVersion()) != nil {
				return nil, errors.New("version mismatch")
			}

			if !bytes.Equal(refData, responses[i].GetData()) {
				return nil, errors.New("data mismatch")
			}
		}
	}

	return responses[0], nil
}

func (n *network) AcceptRecovery(ctx context.Context, handler consensus.RecoveryHandler) {
	if n == nil {
		return
	}

	if handler == nil {
		n.Host.SetStreamHandler(recoveryProtocolID, nil)
		return
	}

	n.Host.SetStreamHandler(recoveryProtocolID, func(s net.Stream) {
		defer func() { _ = s.Reset() }()

		remotePeer := s.Conn().RemotePeer().Pretty()
		buf := bufio.NewReader(s)
		m, err := protocol.Unpack(buf)
		if err != nil {
			zap.L().Warn("RecoveryHandlerRead", zap.String("peer", remotePeer), zap.Error(err))
			return
		}

		req, ok := m.(*consensus.RecoveryRequest)
		if !ok {
			zap.L().Warn("RecoveryHandlerUnpack",
				zap.String("peer", remotePeer),
				zap.Error(errors.New("invalid type")),
			)
			return
		}

		res, err := handler(req)
		if err != nil {
			zap.L().Error("RecoveryHandlerPass", zap.String("peer", remotePeer), zap.Error(err))
			return
		}

		raw, err := protocol.Pack(res)
		if err != nil {
			zap.L().Error("RecoveryHandlerPack", zap.String("peer", remotePeer), zap.Error(err))
			return
		}

		_, err = s.Write(raw)
		if err != nil {
			zap.L().Error("RecoveryHandlerWrite", zap.String("peer", remotePeer), zap.Error(err))
			return
		}

		zap.L().Debug("RecoveryHandler",
			zap.String("key", req.Key),
			zap.String("peer", remotePeer),
		)
	})
}

func (n *network) recoveryStream(ctx context.Context, req []byte, pid peer.ID) (res *consensus.RecoveryResponse) {
	remotePeer := pid.Pretty()

	s, err := n.Host.NewStream(ctx, pid, recoveryProtocolID)
	if err != nil {
		zap.L().Warn("RecoveryStream", zap.String("peer", remotePeer), zap.Error(err))
		return
	}

	_, err = s.Write(req)
	if err != nil {
		zap.L().Error("RecoveryStreamWrite", zap.String("peer", remotePeer), zap.Error(err))
		_ = s.Reset()
		return
	}

	defer func() { _ = s.Reset() }()
	m, err := protocol.Unpack(bufio.NewReader(s))
	if err != nil {
		zap.L().Error("RecoveryStreamUnpack", zap.String("peer", remotePeer), zap.Error(err))
		_ = s.Reset()
		return
	}

	res, ok := m.(*consensus.RecoveryResponse)
	if !ok {
		zap.L().Error("RecoveryStreamUnpack",
			zap.String("peer", remotePeer),
			zap.Error(errors.New("invalid type")),
		)
		return
	}

	return res
}
