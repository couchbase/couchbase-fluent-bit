/*
 *  Copyright 2021 Couchbase, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file  except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the  License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package couchbase

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/couchbase/fluent-bit/pkg/common"
	"go.uber.org/zap/zapcore"
)

type WatcherConfig struct {
	fluentBitConfigDir,
	fluentBitBinaryPath,
	fluentBitConfigFilePath,
	couchbaseLogDir,
	rebalanceOutputDir,
	couchbaseWatchDir string
}

func (cw *WatcherConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("fluentBitConfigDir", cw.fluentBitConfigDir)
	enc.AddString("fluentBitBinaryPath", cw.fluentBitBinaryPath)
	enc.AddString("fluentBitConfigFilePath", cw.fluentBitConfigFilePath)
	enc.AddString("couchbaseLogDir", cw.couchbaseLogDir)
	enc.AddString("rebalanceOutputDir", cw.rebalanceOutputDir)
	enc.AddString("couchbaseWatchDir", cw.couchbaseWatchDir)

	return nil
}

func NewWatcherConfigFromDefaults() *WatcherConfig {
	// Kubesphere configuration
	fluentBitConfigDir := common.GetDynamicConfigDir()
	fluentBitConfigFilePath := common.GetConfigFile()
	fluentBitBinaryPath := common.GetBinaryPath()
	// The logs directory is required to exist
	couchbaseLogDir := common.GetLogsDir()
	// The actual rebalance directory may not exist yet
	couchbaseWatchDir := common.GetRebalanceReportDir()
	// We need write access to this directory
	rebalanceOutputDir := common.GetRebalanceOutputDir()

	config := WatcherConfig{
		fluentBitConfigDir:      fluentBitConfigDir,
		fluentBitBinaryPath:     fluentBitBinaryPath,
		fluentBitConfigFilePath: fluentBitConfigFilePath,
		couchbaseLogDir:         couchbaseLogDir,
		rebalanceOutputDir:      rebalanceOutputDir,
		couchbaseWatchDir:       couchbaseWatchDir,
	}

	log.Infow("Using configuration", "config", config)

	return &config
}

func (cw *WatcherConfig) SetFluentBitConfigDir(value string) {
	cw.fluentBitConfigDir = filepath.Clean(value)
}

func (cw *WatcherConfig) SetCouchbaseLogDir(value string) {
	cw.couchbaseLogDir = filepath.Clean(value)
}

func (cw *WatcherConfig) SetRebalanceOutputDir(value string) {
	cw.rebalanceOutputDir = filepath.Clean(value)
}

func (cw *WatcherConfig) SetCouchbaseWatchDir(value string) {
	cw.couchbaseWatchDir = filepath.Clean(value)
}

func (cw *WatcherConfig) GetFluentBitBinaryPath() string {
	return filepath.Clean(cw.fluentBitBinaryPath)
}

func (cw *WatcherConfig) GetFluentBitConfigFilePath() string {
	return filepath.Clean(cw.fluentBitConfigFilePath)
}

func (cw *WatcherConfig) GetWatchedFluentBitConfigDir() string {
	return filepath.Clean(cw.fluentBitConfigDir)
}

const rebalanceDirPermissions fs.FileMode = 0700

func (cw *WatcherConfig) CreateRebalanceDir() error {
	err := os.Mkdir(cw.rebalanceOutputDir, rebalanceDirPermissions)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("unable to create rebalance output directory %q: %w", cw.rebalanceOutputDir, err)
	}

	return nil
}
