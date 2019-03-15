/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package boltdb provides the (default) BoldDB database driver.
package boltdb

import (
	"errors"
	"sync"

	bolt "github.com/coreos/bbolt"
	"github.com/technicolor-research/pnyxdb/consensus"
)

var bucketName = []byte("pnyxdb")
var errNotFound = errors.New("key corrupted or unknown")

// store is the driver for the BoltDB store engine.
type store struct {
	sync.Mutex

	db *bolt.DB
}

// New generates a new BoltDB store from the storage path.
func New(path string) (consensus.Store, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	s := &store{db: db}

	err = s.db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(bucketName)
		return e
	})

	if err != nil {
		_ = s.Close()
	}

	return s, nil
}

func (s *store) Get(key string) (value []byte, v *consensus.Version, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		data := b.Get([]byte(key))
		if len(data) < consensus.VersionBytes {
			v = consensus.NoVersion
			return errNotFound
		}

		value = make([]byte, len(data[consensus.VersionBytes:]))
		copy(value, data[consensus.VersionBytes:])
		v = &consensus.Version{}
		return v.UnmarshalBinary(data[:consensus.VersionBytes])
	})

	return
}

func (s *store) Set(key string, value []byte, v *consensus.Version) error {
	return s.SetBatch([]string{key}, [][]byte{value}, []*consensus.Version{v})
}

func (s *store) SetBatch(keys []string, values [][]byte, versions []*consensus.Version) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		for i, k := range keys {
			rv, err := versions[i].MarshalBinary()
			if err != nil {
				return err
			}

			err = b.Put([]byte(k), append(rv[:consensus.VersionBytes], values[i]...))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *store) List() (map[string]*consensus.Version, error) {
	catalog := make(map[string]*consensus.Version)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()

		for k, d := c.First(); k != nil; k, d = c.Next() {
			if len(d) >= consensus.VersionBytes {
				v := &consensus.Version{}
				if v.UnmarshalBinary(d[:consensus.VersionBytes]) == nil {
					catalog[string(k)] = v
				}
			}
		}

		return nil
	})

	return catalog, err
}

func (s *store) Close() error {
	return s.db.Close()
}
