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
	}

	for key := range expected {
		if os.Getenv(key) != expected[key] {
			t.Errorf("%q : %q != %q", key, os.Getenv(key), expected[key])
		}

		t.Logf("%q : %q - OK", key, expected[key])
	}
}
