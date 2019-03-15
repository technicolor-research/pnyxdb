/**
 * Copyright (c) 2019 - Present – Thomson Licensing, SAS
 * All rights reserved.
 *
 * This source code is licensed under the Clear BSD license found in the
 * LICENSE file in the root directory of this source tree.
 */

package consensus

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Recover allows to ask the engine to recover one key from other peers.
// This might be useful after being disconnected from the network.
//
// This is an asynchronous process.
func (eng *Engine) Recover(key string) {
	eng.pendingRecovery <- key
}

func (eng *Engine) recoveryHandler(req *RecoveryRequest) (*RecoveryResponse, error) {
	value, version, err := eng.Store.Get(req.GetKey())
	return &RecoveryResponse{
		Key:     req.GetKey(),
		Version: version,
		Data:    value,
	}, err
}

func (eng *Engine) recoveryWorker(ctx context.Context) {
	retry := func(key string) {
		select {
		case eng.pendingRecovery <- key:
		default:
			zap.L().Warn("RecoveryAbort", zap.String("key", key), zap.String("reason", "queueFull"))
		}
	}

	for {
		time.Sleep(time.Second)
		select {
		case key := <-eng.pendingRecovery:
			rec, ok := eng.Network.(RecoveryManager)
			if !ok {
				zap.L().Warn("Recovery", zap.Bool("unsupported", true))
				break
			}

			subctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			res, err := rec.RequestRecovery(subctx, key)
			if err != nil {
				zap.L().Warn("RecoveryRetry", zap.String("key", key), zap.Error(err))
				cancel()
				retry(key)
				break
			}

			eng.Store.Lock()
			err = eng.Store.Set(key, res.GetData(), res.GetVersion())
			eng.Store.Unlock()

			if err != nil {
				zap.L().Warn("RecoveryRetry", zap.String("key", key), zap.Error(err))
				retry(key)
			} else {
				zap.L().Info("RecoverySuccess", zap.String("key", key))
			}
			cancel()

		case <-ctx.Done():
			return
		}
	}
}
