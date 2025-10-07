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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
	ConfigFileEnvVar         = "COUCHBASE_LOGS_CONFIG_FILE"
	configFileDefault        = "fluent-bit.conf"
	binaryEnvVar             = "COUCHBASE_LOGS_BINARY"
	binaryDefault            = "/fluent-bit/bin/fluent-bit"
	logsLocationEnvVar       = "COUCHBASE_LOGS"
	logsLocationDefault      = "/opt/couchbase/var/lib/couchbase/logs/"
	rebalanceLocationEnvVar  = "COUCHBASE_LOGS_REBALANCE_TMP_DIR"
	rebalanceLocationDefault = "/tmp/rebalance-logs"
	bufferLocationEnvVar     = "STORAGE_BUFFER_PATH"
	bufferLocationDefault    = "/tmp/buffer"
	// KubernetesConfigEnvVar should only be used for testing.
	KubernetesConfigEnvVar  = "COUCHBASE_K8S_CONFIG_DIR"
	kubernetesConfigDefault = "/etc/podinfo"
	// Special handling for these annotations.
	FluentBitAnnotationPrefix = "fluentbit.couchbase.com/"
	// Container limits.
	ContainerLimitsMemEnvVar = "CONTAINER_LIMITS_MEMORY_MEGABYTES"
	// Environment variable for if Audit logging is enabled.
	AuditEnabledEnvVar = "AUDIT_ENABLED"
	// Memory Buf limits enabled environment variable.
	MemBufLimitsEnabledEnvVar = "MEM_BUF_LIMITS_ENABLED"
	// Allowance for fluent bit memory estimation.
	FluentBitMemoryMultiplier float32 = 1.2
)

// All default memory buffer limits.
var memoryBufLimits = map[string]string{
	"MBL_AUDIT":      "false",
	"MBL_ERLANG":     "false",
	"MBL_EVENTING":   "false",
	"MBL_HTTP":       "false",
	"MBL_INDEX":      "false",
	"MBL_PROJECTOR":  "false",
	"MBL_JAVA":       "false",
	"MBL_MEMCACHED":  "false",
	"MBL_PROMETHEUS": "false",
	"MBL_REBALANCE":  "false",
	"MBL_XDCR":       "false",
	"MBL_QUERY":      "false",
	"MBL_FTS":        "false",
}

func GetStorageBufferDir() string {
	return GetDirectory(bufferLocationDefault, bufferLocationEnvVar)
}

func GetDynamicConfigDir() string {
	return GetDirectory(dynamicConfigDefault, DynamicConfigEnvVar)
}

func GetConfigFile() string {
	fluentBitConfigDir := GetDynamicConfigDir()

	return GetDirectory(filepath.Join(fluentBitConfigDir, configFileDefault), ConfigFileEnvVar)
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

func GetAuditEnabled() bool {
	auditEnabled, _ := strconv.ParseBool(os.Getenv(AuditEnabledEnvVar))

	return auditEnabled
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

	SetBufferDirectory()

	log.Infow("Loaded environment files")
	processCouchbaseAnnotations()
}

func setMemoryBufLimitDefaults() {
	for k, v := range memoryBufLimits {
		os.Setenv(k, v)
	}
}

func SetBufferDirectory() {
	// set where overflowed buffers should write to.
	bufferDirectory := GetStorageBufferDir()
	os.Setenv(bufferLocationEnvVar, bufferDirectory)
}

// memoryBufferLimitEnabled parses env variable `MEM_BUF_LIMITS_ENABLED`
// returns truthy value or panics if it fails to parse.
func memoryBufferLimitEnabled() bool {
	memBuflimitEnabledEnvValue := os.Getenv(MemBufLimitsEnabledEnvVar)
	if memBuflimitEnabledEnvValue != "" {
		value, err := strconv.ParseBool(memBuflimitEnabledEnvValue)
		if err != nil {
			log.Fatalw("Failed to convert memory buffer limit var into bool", MemBufLimitsEnabledEnvVar, memBuflimitEnabledEnvValue)
		}

		return value
	}

	return false
}

// Sets memory buffer limits if enabled.
func CheckAndEnableMemoryBufLimits() {
	setMemoryBufLimitDefaults()

	if memoryBufferLimitEnabled() {
		updateMemoryBufLimits()
	}
}

// updateDefaultMemoryBufLimits calculates the.
func updateMemoryBufLimits() {
	memoryLimit := os.Getenv(ContainerLimitsMemEnvVar)
	if memoryLimit == "" {
		setMemoryBufLimitDefaults()

		return
	}

	memoryMB, err := strconv.Atoi(memoryLimit)

	if err != nil {
		log.Fatalw("Unable to convert environment memory limit to int", ContainerLimitsMemEnvVar, memoryLimit)
	}

	fbConfig, err := BuildConfigFile(GetConfigFile())
	if err != nil {
		log.Fatalw("Failed to parse fb config file", "error", err)
	}

	memBufConfig := CreateMemBufLimitConfig(fbConfig)

	if memBufConfig.NumInputs == 0 && memBufConfig.NumOutputs == 0 {
		log.Info("No input or output plugins found, not updating memory buffer limits")

		return
	}

	if len(memBufConfig.MemBufLimitNames) == 0 {
		log.Info("No ${MBL_*} variables found, not updating memory buffer limits")

		return
	}

	perInputMemLimit := calculatePerInputMemoryLimit(memoryMB, memBufConfig.NumInputs, memBufConfig.NumOutputs)

	megabyteString := fmt.Sprintf("%dMB", perInputMemLimit)
	log.Info("Setting new memory buffer limits ", perInputMemLimit)

	for _, k := range memBufConfig.MemBufLimitNames {
		os.Setenv(k, megabyteString)
	}
}

// https://docs.fluentbit.io/manual/v/1.0/configuration/memory_usage#estimating
func calculatePerInputMemoryLimit(totalMemory, inputs, outputs int) int {
	estimatedMemoryUsage := float32(inputs+2*outputs) * FluentBitMemoryMultiplier

	return totalMemory / int(estimatedMemoryUsage)
}

// Some extra processing of specific "fluentbit.couchbase.com" annotations ones:
// Remove prefix, uppercase and replace dots, etc. with underscores.
// Intention is to simplify usage by having support for all shells and short env vars.
func processCouchbaseAnnotations() {
	// Anything that is not a letter or underscore is replaced with an underscore
	re := regexp.MustCompile(`\W`)

	// We should never have more than 2 splits around the = sign.
	// Required for linting.
	const maxSplit = 2

	for _, pair := range os.Environ() {
		if strings.HasPrefix(pair, FluentBitAnnotationPrefix) {
			newPair := strings.Split(strings.TrimPrefix(pair, FluentBitAnnotationPrefix), "=")
			if len(newPair) > maxSplit {
				log.Warnw("Unable to split correctly", "value", pair, "size", len(newPair))

				continue
			}

			// Make sure we uppercase the key and remove any special characters
			key := re.ReplaceAllString(strings.ToUpper(newPair[0]), "_")

			value := ""
			if len(newPair) > 1 {
				value = newPair[1]
			}

			// Finally update
			os.Setenv(key, value)
			log.Infow("Parsed special annotation pair into new variable", "original", pair, "new key", key, "new value", value)
		}
	}
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
