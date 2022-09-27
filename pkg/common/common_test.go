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
package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/couchbase/fluent-bit/pkg/common"
)

func TestGetDirectory(t *testing.T) {
	t.Parallel()

	testKey := "TEST_GET_DEFAULT"
	expected := "DEFAULT"

	value := common.GetDirectory(expected, testKey)
	if value != expected {
		t.Errorf("%q != %q", value, expected)
	}

	expected = "/Something/other/than/the/default////"
	os.Setenv(testKey, expected)

	value = common.GetDirectory(expected, testKey)
	if value == expected {
		t.Errorf("%q == %q", value, expected)
	}

	if value != filepath.Clean(expected) {
		t.Errorf("%q != %q", value, filepath.Clean(expected))
	}
}

func TestLoadEnvironment(t *testing.T) {
	t.Parallel()

	os.Setenv(common.DynamicConfigEnvVar, "testdata/dynamic")
	os.Setenv(common.KubernetesConfigEnvVar, "testdata/kubernetes")

	if common.GetDynamicConfigDir() != "testdata/dynamic" {
		t.Errorf("%q != %q", common.GetDynamicConfigDir(), "testdata/dynamic")
	}

	if common.GetKubernetesConfigDir() != "testdata/kubernetes" {
		t.Errorf("%q != %q", common.GetKubernetesConfigDir(), "testdata/kubernetes")
	}

	common.LoadEnvironment()

	expected := map[string]string{
		"OVERRIDE_ME":           "user",
		"NESTED":                "true",
		"KUBERNETES_ANNOTATION": "true",
		"KUBERNETES_LABEL":      "true",
		"KUBERNETES_LABEL2":     "false",
		// Special annotation processing, only key should be uppercase
		"ENABLE_LOKI": "true",
		"LOKI_HOST":   "loki.test",
	}

	for key := range expected {
		if os.Getenv(key) != expected[key] {
			t.Errorf("%q : %q != %q", key, os.Getenv(key), expected[key])
		} else {
			t.Logf("%q : %q - OK", key, expected[key])
		}
	}
}
func TestHandlesEnvMemBufLimits(t *testing.T) {
	t.Parallel()
	os.Setenv(common.ContainerLimitsMemEnvVar, "1000")
	os.Setenv(common.MemBufLimitsEnabledEnvVar, "true")

	var keys = [...]string{"MBL_AUDIT", "MBL_ERLANG", "MBL_EVENTING", "MBL_HTTP", "MBL_INDEX", "MBL_PROJECTOR", "MBL_JAVA", "MBL_MEMCACHED", "MBL_PROMETHEUS", "MBL_REBALANCE", "MBL_XDCR", "MBL_QUERY", "MBL_FTS"}

	// test with audit enabled
	var expected = "35MB"

	os.Setenv(common.AuditEnabledEnvVar, "true")

	os.Setenv(common.ConfigFileEnvVar, "../../test/test-fluent-bit.conf")

	os.Setenv("STDOUT_MATCH", "*")
	os.Setenv("ES_MATCH", "*")

	common.CheckAndEnableMemoryBufLimits()

	for _, key := range keys {
		if os.Getenv(key) != expected {
			t.Errorf("%q : %q != %q", key, os.Getenv(key), expected)
		} else {
			t.Logf("%q : %q - OK", key, expected)
		}

		os.Setenv(key, "")
	}

	// test with audit disabled
	expected = "37MB"

	os.Setenv(common.AuditEnabledEnvVar, "false")

	common.CheckAndEnableMemoryBufLimits()

	for _, key := range keys {
		if os.Getenv(key) != expected {
			t.Errorf("%q : %q != %q", key, os.Getenv(key), expected)
		} else {
			t.Logf("%q : %q - OK", key, expected)
		}

		os.Setenv(key, "")
	}

	// Test if no memory buffer limits exist to be set
	os.Setenv(common.ConfigFileEnvVar, "../../test/example/test-fluent-bit-simple.conf")
	common.CheckAndEnableMemoryBufLimits()

	expected = "false"

	for _, key := range keys {
		if os.Getenv(key) != expected {
			t.Errorf("%q : %q != %q", key, os.Getenv(key), expected)
		} else {
			t.Logf("%q : %q - OK", key, expected)
		}
	}
}
