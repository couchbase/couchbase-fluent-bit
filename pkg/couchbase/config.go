package couchbase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/couchbase/fluent-bit/pkg/common"
)

type WatcherConfig struct {
	fluentBitConfigDir,
	fluentBitBinaryPath,
	fluentBitConfigFilePath,
	couchbaseLogDir,
	rebalanceOutputDir,
	couchbaseWatchDir string
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

func (cw *WatcherConfig) CreateRebalanceDir() error {
	err := os.Mkdir(cw.rebalanceOutputDir, 0700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("unable to create rebalance output directory %q: %w", cw.rebalanceOutputDir, err)
	}

	return nil
}
