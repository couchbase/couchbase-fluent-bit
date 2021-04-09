package couchbase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	fluentBitConfigDir := GetDirectory("/fluent-bit/config", "COUCHBASE_LOGS_DYNAMIC_CONFIG")
	fluentBitConfigFilePath := GetDirectory(filepath.Join(fluentBitConfigDir, "fluent-bit.conf"), "COUCHBASE_LOGS_CONFIG_FILE")
	fluentBitBinaryPath := GetDirectory("/fluent-bit/bin/fluent-bit", "COUCHBASE_LOGS_BINARY")
	// The logs directory is required to exist
	couchbaseLogDir := GetDirectory("/opt/couchbase/var/couchbase/logs", "COUCHBASE_LOGS")
	// The actual rebalance directory may not exist yet
	couchbaseWatchDir := filepath.Join(couchbaseLogDir, "rebalance")
	// We need write access to this directory
	rebalanceOutputDir := GetDirectory("/tmp/rebalance-logs", "COUCHBASE_LOGS_REBALANCE_TEMPDIR")

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
