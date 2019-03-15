/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package gossipsub is a wrapper to connect PnyxDB nodes with libp2p gossip.
package gossipsub

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	floodsub "github.com/libp2p/go-floodsub"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/network/protocol"
	"go.uber.org/zap"
)

func init() {
	floodsub.GossipSubHistoryGossip = 256
	floodsub.GossipSubHistoryLength = 1024
}

// Parameters holds gossipsub instance parameters.
type Parameters struct {
	Host           host.Host
	Topic          string
	BootstrapAddrs []string
	ChannelsBuffer uint
	RecoveryQuorum uint

	Ctx context.Context
}

// Defaults return safe defaults for gossipsub.
func Defaults(h host.Host) Parameters {
	return Parameters{
		Host:           h,
		Topic:          "pnyxdb",
		ChannelsBuffer: 1024,
		RecoveryQuorum: 3,
		Ctx:            context.Background(),
	}
}

type network struct {
	sync.RWMutex
	Parameters
	*floodsub.PubSub

	pending   []proto.Message
	acceptors []consensus.MessageAcceptor
	receivers []chan proto.Message
	cancel    context.CancelFunc
	rand      *rand.Rand
}

// New returns a new gossipsub-based network.
func New(p Parameters) (consensus.Network, error) {
	mainCtx, cancel := context.WithCancel(p.Ctx)

	for _, addr := range p.BootstrapAddrs {
		// addr format:
		// /ip4/<ip>/tcp/<port>/p2p/<peer id>

		addr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			cancel()
			return nil, err
		}

		rawPeerID, err := addr.ValueForProtocol(multiaddr.P_P2P)
		if err != nil {
			cancel()
			return nil, err
		}

		peerID, err := peer.IDB58Decode(rawPeerID)
		if err != nil {
			cancel()
			return nil, err
		}

		targetPeerAddr, _ := multiaddr.NewMultiaddr("/ipfs/" + rawPeerID)
		targetAddr := addr.Decapsulate(targetPeerAddr)

		go func() {
			var connected bool
			for { // Periodically ensure connection to peer
				err := p.Host.Connect(mainCtx, peerstore.PeerInfo{
					ID:    peerID,
					Addrs: []multiaddr.Multiaddr{targetAddr},
				})

				if err == nil && !connected {
					zap.L().Info("Connected",
						zap.String("address", targetAddr.String()),
					)
					/*} else if err != nil {
					zap.L().Warn("Connection error",
						zap.String("address", targetAddr.String()),
						zap.Error(err),
					)*/
				}
				connected = err == nil

				select {
				case <-time.After(5 * time.Second):
				case <-mainCtx.Done():
					return
				}
			}
		}()
	}

	gs, err := floodsub.NewGossipSub(p.Ctx, p.Host)
	if err != nil {
		cancel()
		return nil, err
	}

	subscription, err := gs.Subscribe(p.Topic)
	if err != nil {
		cancel()
		return nil, err
	}

	n := &network{
		Parameters: p,
		PubSub:     gs,
		cancel:     cancel,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	go n.run(mainCtx, subscription)
	return n, nil
}

func (n *network) run(ctx context.Context, s *floodsub.Subscription) {
	defer s.Cancel()
	for {
		raw, err := s.Next(ctx)
		if err != nil {
			// TODO add log
			fmt.Println("TODO ERROR 1", err)
			return
		}

		m, err := protocol.Unpack(bytes.NewBuffer(raw.Data))
		if err != nil {
			// TODO add log
			fmt.Println("TODO ERROR 2", err)
			continue
		}

		n.RLock()
		var delivered bool
		for i, acceptor := range n.acceptors {
			if acceptor(m) {
				n.receivers[i] <- m
				delivered = true
			}
		}
		n.RUnlock()

		if !delivered {
			n.Lock()
			n.pending = append(n.pending, m)
			n.Unlock()
		}
	}
}

func (n *network) Accept(ctx context.Context, acceptor consensus.MessageAcceptor) <-chan proto.Message {
	input := make(chan proto.Message, n.Parameters.ChannelsBuffer)
	output := make(chan proto.Message)

	n.Lock()
	defer n.Unlock()

	n.acceptors = append(n.acceptors, acceptor)
	n.receivers = append(n.receivers, input)

	// Consume pending messages if possible
	var toSend []proto.Message
	newPending := n.pending[:0]
	for _, m := range n.pending {
		if acceptor(m) {
			toSend = append(toSend, m)
		} else {
			newPending = append(newPending, m)
		}
	}

	n.pending = newPending

	// Run in a routine to avoid locking if many released pending messages
	go func() {
		for _, p := range toSend {
			select {
			case input <- p:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer close(output)

		var done bool
		for !done {
			select {
			case m := <-input:
				select {
				case output <- m:
				case <-ctx.Done():
					done = true
				}
			case <-ctx.Done():
				done = true
			}
		}

		n.Lock()
		defer n.Unlock()

		for i, r := range n.receivers {
			if r == input {
				n.acceptors = append(n.acceptors[:i], n.acceptors[i+1:]...)
				n.receivers = append(n.receivers[:i], n.receivers[i+1:]...)
				return
			}
		}
	}()

	return output
}

func (n *network) Broadcast(m proto.Message) error {
	raw, err := protocol.Pack(m)
	if err != nil {
		return err
	}

	return n.Publish(n.Parameters.Topic, raw)
}

func (n *network) Close() error {
	n.cancel()
	return nil
}
