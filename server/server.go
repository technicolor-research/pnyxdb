/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package server provides PnyxDB API server.
package server

import (
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/technicolor-research/pnyxdb/api"
	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/consensus/encoding"
)

// Server is the GRPC PnyxDB endpoint.
type Server struct {
	*consensus.Engine
	Listen string
}

// Get gets a value from the database.
func (s *Server) Get(ctx context.Context, key *api.Key) (*api.Value, error) {
	value, version, err := s.Store.Get(key.Key)
	return &api.Value{
		Version: version,
		Data:    value,
	}, err
}

// Members returns the members of a specific set.
func (s *Server) Members(ctx context.Context, key *api.Key) (*api.Values, error) {
	value, version, err := s.Store.Get(key.Key)
	if err != nil {
		return nil, err
	}

	set := encoding.NewSet()
	err = set.UnmarshalBinary(value)
	if err != nil {
		return nil, err
	}

	values := &api.Values{
		Version: version,
	}

	for key := range set.Elements {
		values.Data = append(values.Data, []byte(key))
	}
	return values, nil
}

// Contains returns whether a particular set contains a specific value or not.
func (s *Server) Contains(ctx context.Context, kv *api.KeyValue) (*api.Boolean, error) {
	value, _, err := s.Store.Get(kv.Key)
	if err != nil {
		return nil, err
	}

	set := encoding.NewSet()
	err = set.UnmarshalBinary(value)
	if err != nil {
		return nil, err
	}

	return &api.Boolean{Boolean: set.Contains(kv.Value)}, nil
}

// Submit submits a set of operations to the database.
func (s *Server) Submit(ctx context.Context, tx *api.Transaction) (*api.Receipt, error) {
	query := consensus.NewQuery()
	query.Policy = tx.Policy
	query.Requirements = tx.Requirements
	query.Operations = tx.Operations
	query.Deadline = tx.Deadline

	return &api.Receipt{Uuid: query.Uuid}, s.Engine.Submit(query)
}

// Serve starts the PnyxDB GRPC server for clients.
func (s *Server) Serve() error {
	lis, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return err
	}

	srv := grpc.NewServer()
	api.RegisterEndorserServer(srv, s)
	return srv.Serve(lis)
}
