/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const httpLogSinkCapacity = 512 * 1024 // 512 KiB

type httpLogSink struct {
	sync.Mutex
	address string
	buffer  *bytes.Buffer
}

func init() {
	err := zap.RegisterSink("http", httpLogSinkFactory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to register httpLogSink:", err)
	}
}

var loggerCmd = &cobra.Command{
	Use:   "logger <listen>",
	Short: "Run a centralized logger",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var m sync.Mutex
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.NotFound(w, r)
				return
			}

			m.Lock()
			defer m.Unlock()
			_, _ = io.Copy(os.Stdout, r.Body)
		})

		_ = http.ListenAndServe(args[0], nil)
	},
}

func httpLogSinkFactory(u *url.URL) (zap.Sink, error) {
	b := make([]byte, 0, httpLogSinkCapacity)
	return &httpLogSink{
		address: u.String(),
		buffer:  bytes.NewBuffer(b),
	}, nil
}

func (s *httpLogSink) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()

	if len(p)+s.buffer.Len() > s.buffer.Cap() {
		err = s.drain()
		if err != nil {
			return
		}
	}

	return s.buffer.Write(p)
}

func (s *httpLogSink) Sync() error {
	s.Lock()
	defer s.Unlock()
	return s.drain()
}

func (s *httpLogSink) Close() error {
	return s.Sync()
}

func (s *httpLogSink) drain() error {
	res, err := http.Post(s.address, "application/json", s.buffer)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return errors.New("invalid error code")
	}

	return nil
}

func init() {
	RootCmd.AddCommand(loggerCmd)
}
