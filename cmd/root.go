/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

// Package cmd provides PnyxDB CLI interface.
package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var cfgFile *string
var cfgErr error

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "pnyxdb",
	Short: "PnyxDB, a Lightweight Leaderless Democratic Byzantine Fault Tolerant Consortium Database.",
	Long:  ``,
}

func init() {
	cobra.OnInitialize(initConfig)
	cfgFile = RootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is ./config.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if *cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(*cfgFile)
	} else {
		viper.SetConfigName("config") // name of config file (without extension)
		viper.AddConfigPath(".")      // adding home directory as first search path
	}

	// If a config file is found, read it in.
	var err = viper.ReadInConfig()
	if err != nil {
		cfgErr = err
	}

	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv() // read in environment variables that match

	// Put default values
	if !viper.IsSet("db.driver") {
		viper.Set("db.driver", "boltdb")
	}

	// Init logging
	logEncoder := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	logConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:       true,
		DisableStacktrace: true,
		DisableCaller:     true,
		Encoding:          "console",
		EncoderConfig:     logEncoder,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	if viper.IsSet("machinelog") {
		logConfig.Encoding = "json"
		logConfig.EncoderConfig = zap.NewProductionEncoderConfig()
		logConfig.OutputPaths = []string{viper.GetString("machinelog")}
		logConfig.InitialFields = map[string]interface{}{
			"identity": viper.GetString("identity"),
		}
	}

	l, err := logConfig.Build()
	if err != nil {
		cfgErr = err
	} else {
		zap.ReplaceGlobals(l)
	}
}
