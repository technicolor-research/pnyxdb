/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package boltdb

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/technicolor-research/pnyxdb/consensus"
)

var ts *store

func TestMain(m *testing.M) {
	path, err := ioutil.TempDir("", "pnyxdb_boltdb_")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tsInterface, err := New(filepath.Join(path, "db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ts = tsInterface.(*store)

	res := m.Run()

	_ = ts.Close()
	_ = os.RemoveAll(path)
	os.Exit(res)
}

func TestS_PutGet(t *testing.T) {
	k := "testSet"
	cases := [][]byte{
		[]byte("Hello world!"),
		{},
		make([]byte, 4*1024*1024),
	}

	for _, d := range cases {
		v := consensus.NewVersion(d)
		err := ts.Set(k, d, v)
		require.Nil(t, err)

		d2, v2, err := ts.Get(k)
		require.Nil(t, err)
		require.Exactly(t, d, d2)
		require.Exactly(t, v, v2)
	}
}

func TestS_PutGetBatch(t *testing.T) {
	keys := []string{
		"testBatch_a",
		"testBatch_b",
		"testBatch_c",
	}
	values := [][]byte{
		[]byte("Hello"),
		[]byte("World!"),
		{},
	}

	versions := make([]*consensus.Version, len(keys))
	for i, v := range values {
		versions[i] = consensus.NewVersion(v)
	}

	require.Nil(t, ts.SetBatch(keys, values, versions))
	for i, k := range keys {
		value, v, err := ts.Get(k)
		require.Nil(t, err)
		require.Nil(t, v.Matches(versions[i]))
		require.Exactly(t, values[i], value)
	}
}

func TestS_Get_Unknown(t *testing.T) {
	_, v, err := ts.Get("testUnknown")
	require.NotNil(t, err)
	require.Exactly(t, v, consensus.NoVersion)
}

func TestS_List(t *testing.T) {
	d := []byte("Content")
	v := consensus.NewVersion(d)
	_ = ts.Set("testList", d, v)

	catalog, err := ts.List()
	require.Nil(t, err)
	require.Len(t, catalog, 5)
	require.Contains(t, catalog, "testSet")
	require.Contains(t, catalog, "testBatch_a")
	require.Contains(t, catalog, "testBatch_b")
	require.Contains(t, catalog, "testBatch_c")
	require.Exactly(t, catalog["testList"], v)
}
