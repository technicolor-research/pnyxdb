/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	metrics "github.com/libp2p/go-libp2p-metrics"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/technicolor-research/pnyxdb/consensus"
	"github.com/technicolor-research/pnyxdb/consensus/bbc"
	"github.com/technicolor-research/pnyxdb/network/gossipsub"
	"github.com/technicolor-research/pnyxdb/server"
	"github.com/technicolor-research/pnyxdb/storage/boltdb"
)

type driverConstructor func(string) (consensus.Store, error)

var fullSync *string
var dumpFile *string
var recoveryKeys *[]string
var storeDrivers map[string]driverConstructor

func init() {
	addDriver("boltdb", boltdb.New)
}

func addDriver(name string, c driverConstructor) {
	if storeDrivers == nil {
		storeDrivers = make(map[string]driverConstructor)
	}

	storeDrivers[name] = c
}

func getDriver(name string, path string) (consensus.Store, error) {
	if storeDrivers == nil || storeDrivers[name] == nil {
		fmt.Fprintln(os.Stderr, "Available database drivers:")
		for k := range storeDrivers {
			fmt.Fprintln(os.Stderr, "  *", k)
		}
		return nil, errors.New("unknown database driver: " + name)
	}

	return storeDrivers[name](path)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run a PnyxDB node",
	Run: func(cmd *cobra.Command, args []string) {
		check(cfgErr)
		n := viper.GetInt("n") // TODO clean policy
		w := viper.GetInt("w") // TODO clean policy

		store, err := getDriver(viper.GetString("db.driver"), viper.GetString("db.path"))
		check(err)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			c := make(chan os.Signal, 2)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			for range c {
				cancel()
				_ = store.Close()
				_ = zap.L().Sync()
				memguard.SafeExit(0)
			}
		}()

		keyRing := getKeyRing()
		check(keyRing.UnlockPrivate(getPassword()))

		sk, err := crypto.UnmarshalEd25519PrivateKey(keyRing.GetPrivate())
		check(err)

		reporter := metrics.NewBandwidthCounter()

		host, err := libp2p.New(
			ctx,
			libp2p.Identity(sk),
			libp2p.ListenAddrStrings(viper.GetString("p2p.listen")),
			libp2p.BandwidthReporter(reporter),
		)
		check(err)
		params := gossipsub.Defaults(host)
		params.BootstrapAddrs = viper.GetStringSlice("p2p.peers")
		rq := viper.GetInt("recoveryQuorum")
		if rq > 0 {
			params.RecoveryQuorum = uint(rq)
		}

		network, err := gossipsub.New(params)
		check(err)

		go startReporter(ctx, reporter)

		for _, addr := range host.Addrs() {
			zap.L().Info("Listening",
				zap.String("type", "P2P"),
				zap.String("address", addr.String()+"/p2p/"+host.ID().Pretty()),
			)
		}

		ve, err := bbc.NewVetoEngine(network, keyRing, n)
		check(err)

		engine := consensus.NewEngine(store, network, ve, keyRing, w)

		if *dumpFile != "" {
			check(loadDump(engine))
			go startDumper(ctx, engine)
		}

		check(engine.Run(ctx))

		srv := &server.Server{
			Engine: engine,
			Listen: viper.GetString("api.listen"),
		}

		zap.L().Info("Listening",
			zap.String("type", "API"),
			zap.String("address", viper.GetString("api.listen")),
		)

		go startRecovery(engine)
		err = srv.Serve()

		if err != nil {
			zap.L().Error("Unable to listen",
				zap.String("type", "API"),
				zap.Error(err),
			)
		}
	},
}

func startReporter(ctx context.Context, reporter *metrics.BandwidthCounter) {
	for {
		select {
		case <-time.After(10 * time.Second):
			s := reporter.GetBandwidthTotals()
			/*zap.L().Debug("BandwidthTotal",
				zap.Int64("in", s.TotalIn),
				zap.Int64("out", s.TotalOut),
			)
			*/
			zap.L().Info("Bandwidth",
				zap.Float64("in", s.RateIn),
				zap.Float64("out", s.RateOut),
			)
		case <-ctx.Done():
			return
		}
	}
}

func startDumper(ctx context.Context, e *consensus.Engine) {
	for {
		select {
		case <-e.ActivityProbe:
		case <-time.After(time.Minute):
		case <-ctx.Done():
			return
		}

		from := time.Now()
		file, err := os.Create(*dumpFile)
		if err != nil {
			zap.L().Error("DumpCreate",
				zap.Error(err),
			)
			time.Sleep(5 * time.Second) // backoff to avoid infinite loops
			continue
		}

		err = e.Dump(file)
		_ = file.Close()
		if err != nil {
			zap.L().Error("DumpWrite",
				zap.Error(err),
			)
			continue
		}

		zap.L().Debug("DumpWrite", zap.Duration("duration", time.Since(from)))
	}
}

func loadDump(e *consensus.Engine) error {
	file, err := os.Open(*dumpFile)
	if err != nil {
		zap.L().Warn("DumpFile",
			zap.Error(err),
		)
		return nil
	}

	err = e.Load(file)
	_ = file.Close()
	return err
}

func startRecovery(eng *consensus.Engine) {
	// Wait for startup
	time.Sleep(10 * time.Second)
	for _, key := range *recoveryKeys {
		eng.Recover(key)
	}
}

func init() {
	RootCmd.AddCommand(serverCmd)

	fullSync = serverCmd.Flags().StringP("full-sync", "s", "", "identity of peer to ask for a full state-transfer")
	dumpFile = serverCmd.Flags().StringP("dump", "d", ".dump.p", "file used to retrieve processus state")
	recoveryKeys = serverCmd.Flags().StringSliceP(
		"recover", "r", nil, "set of keys to recover at startup from random peers")
}
