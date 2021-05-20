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

package common

import (
	"os"
	"path"
	"path/filepath"

	"github.com/couchbase/fluent-bit/pkg/logging"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
)

var (
	log = logging.Log
)

const (
	// DynamicConfigEnvVar should only be used for testing.
	DynamicConfigEnvVar      = "COUCHBASE_LOGS_DYNAMIC_CONFIG"
	dynamicConfigDefault     = "/fluent-bit/config"
	configFileEnvVar         = "COUCHBASE_LOGS_CONFIG_FILE"
	configFileDefault        = "fluent-bit.conf"
	binaryEnvVar             = "COUCHBASE_LOGS_BINARY"
	binaryDefault            = "/fluent-bit/bin/fluent-bit"
	logsLocationEnvVar       = "COUCHBASE_LOGS"
	logsLocationDefault      = "/opt/couchbase/var/lib/couchbase/logs/"
	rebalanceLocationEnvVar  = "COUCHBASE_LOGS_REBALANCE_TEMPDIR"
	rebalanceLocationDefault = "/tmp/rebalance-logs"
	// KubernetesConfigEnvVar should only be used for testing.
	KubernetesConfigEnvVar  = "COUCHBASE_K8S_CONFIG_DIR"
	kubernetesConfigDefault = "/etc/podinfo"
)

func GetDynamicConfigDir() string {
	return GetDirectory(dynamicConfigDefault, DynamicConfigEnvVar)
}

func GetConfigFile() string {
	fluentBitConfigDir := GetDynamicConfigDir()

	return GetDirectory(filepath.Join(fluentBitConfigDir, configFileDefault), configFileEnvVar)
}

func GetBinaryPath() string {
	return GetDirectory(binaryDefault, binaryEnvVar)
}

func GetLogsDir() string {
	return GetDirectory(logsLocationDefault, logsLocationEnvVar)
}

func GetRebalanceReportDir() string {
	couchbaseLogDir := GetLogsDir()

	return filepath.Join(couchbaseLogDir, "rebalance")
}

func GetRebalanceOutputDir() string {
	return GetDirectory(rebalanceLocationDefault, rebalanceLocationEnvVar)
}

func GetKubernetesConfigDir() string {
	return GetDirectory(kubernetesConfigDefault, KubernetesConfigEnvVar)
}

// LoadEnvironment is responsible for pulling in any extra information about the environment from various configuration files.
// This is to simplify usage across kubernetes and other deployments.
func LoadEnvironment() {
	// Pick up the generic kubernetes location and (attempt to) load any files there
	_ = filepath.Walk(GetKubernetesConfigDir(),
		func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !f.IsDir() {
				_ = godotenv.Overload(path)
			}

			return nil
		})

	// Support overriding via a file in the mounted directory directly:
	_ = godotenv.Overload(filepath.Join(GetDynamicConfigDir(), "config.env"))

	log.Infow("Loaded environment files")
}

func GetDirectory(defaultValue, environmentVariable string) string {
	directoryName := os.Getenv(environmentVariable)
	if directoryName == "" {
		log.Infow("No environment variable so defaulting", "environmentVariable", environmentVariable, "defaultValue", defaultValue)
		directoryName = defaultValue
	}

	return path.Clean(directoryName)
}

func IsValidEvent(event fsnotify.Event) bool {
	// Inspired by https://github.com/jimmidyson/configmap-reload
	return event.Op&fsnotify.Create == fsnotify.Create
}
