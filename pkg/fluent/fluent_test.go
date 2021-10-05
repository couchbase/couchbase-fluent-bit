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

package fluent_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/couchbase/fluent-bit/pkg/fluent"
	"github.com/couchbase/fluent-bit/pkg/logging"
	"github.com/oklog/run"
)

var (
	log = logging.GetLogger("fluent-bit-tester")
)

func createConfigTestDir(t *testing.T, baseDir, testName string) string {
	t.Helper()

	dir, err := ioutil.TempDir(baseDir, testName)
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func testFileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func getCustomCommand(testFile string) string {
	return "rm -f " + testFile + ";sleep 5; touch " + testFile
}

// Check we can run a custom binary and verify it does run to completion.
func TestCommandRun(t *testing.T) {
	t.Parallel()

	// Use a temporary directory to watch
	dir := createConfigTestDir(t, "", "command_run_test")
	defer os.RemoveAll(dir)

	testFile := filepath.Join(dir, "test.exists")

	// Use a custom binary, i.e. sleep, for testing
	config := fluent.NewFluentBitConfig("/bin/bash", getCustomCommand(testFile), dir, "", "")

	if testFileExists(testFile) {
		t.Error("Test file already exists at the start")
	}

	fluent.Start(config)
	fluent.Wait(config)

	if !testFileExists(testFile) {
		t.Error("Unable to find expected test output file")
	}
}

// Check we fail to start appropriately as well.
func TestCommandFailedRun(t *testing.T) {
	t.Parallel()

	config := fluent.NewFluentBitConfig("/iamrootandunabletodoanything", "invalidconfig", "/i/do/not/exist", "", "")
	fluent.Start(config)

	if config.IsCleanStart() {
		t.Error("Incorrectly started with invalid configuration")
	}
}

// Check that we can actually stop the binary.
func TestCommandStop(t *testing.T) {
	t.Parallel()

	// Run forever (well a long time)
	config := fluent.NewFluentBitConfig("/bin/bash", "sleep 10000", "", "", "")

	// Explicit timeout
	timeout := time.After(time.Second)
	done := make(chan bool)

	go func() {
		fluent.Start(config)
		fluent.Stop(config)
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test was not stopped")
	case <-done:
		log.Info("Correctly ended")
	}
}

// Confirm that we can watch for config changes and FB gets restarted then.
func TestFluentBitRestartOnConfigChange(t *testing.T) {
	t.Parallel()
	// Use a temporary directory to watch
	dir := createConfigTestDir(t, "", "fluent_bit_restart_test")
	defer os.RemoveAll(dir)

	testFile := filepath.Join(dir, "test.restarts")

	// Use a custom binary, i.e. sleep, for testing
	config := fluent.NewFluentBitConfig("/bin/bash", "sleep 20000", dir, "", "")

	var g run.Group
	if err := fluent.AddDynamicConfigWatcher(&g, config); err != nil {
		t.Fatal(err)
	}

	if testFileExists(testFile) {
		t.Error("Test file already exists at the start")
	}

	if config.GetStartCount() != 0 {
		t.Errorf("Invalid restart count at the start: %d", config.GetStartCount())
	}

	// We add a file to the config directory and it should restart the command
	g.Add(func() error {
		for i := 1; i <= 5; i++ {
			// Allow for command to start before we change anything
			time.Sleep(time.Second)

			if config.GetStartCount() != i {
				t.Errorf("Invalid restart count before creating file: %d != %d", config.GetStartCount(), i)
			}

			// We need to make sure we are watching for the file before we create it
			dst, err := os.Create(filepath.Join(dir, filepath.Base("test_file_"+strconv.Itoa(i))))
			if err != nil {
				t.Fatal(err, i)
			}
			// Make sure we close it straight away to flush
			_ = dst.Close()

			// Allow us time to restart
			time.Sleep(time.Second)

			// Check we have incremented the start count
			if config.GetStartCount() != i+1 {
				t.Errorf("Invalid restart count: %d != %d", config.GetStartCount(), i+1)
			}
		}

		return nil
	}, func(err error) {
		if err != nil {
			t.Errorf("Error during test: %v", err)
		}
	})

	if err := g.Run(); err != nil {
		t.Errorf("Error during test: %v", err)
	}
}

func TestAddDynamicConfig(t *testing.T) {
	t.Parallel()

	defaultConfig := filepath.Clean("../../conf/fluent-bit.conf")
	couchbaseConfigDir := filepath.Clean("../../conf/couchbase")

	// None of these should generate a custom configuration
	for _, testvalue := range []string{"stdout", "stdout,stdout", "stdout,", ",,"} {
		actual, err := fluent.AddDynamicConfig(defaultConfig, couchbaseConfigDir, testvalue)

		if err != nil {
			t.Errorf("%s: %q", testvalue, err.Error())
		}

		if actual != defaultConfig {
			t.Errorf("%s: %q != %q", testvalue, actual, defaultConfig)
		}
	}

	// Now test for Loki (extend as we add others)
	for _, testvalue := range []string{"stdout,loki", "loki,stdout", "Loki"} {
		actual, err := fluent.AddDynamicConfig(defaultConfig, couchbaseConfigDir, testvalue)
		defer os.Remove(actual)

		if err != nil {
			t.Fatal(err)
		}

		if actual == defaultConfig {
			t.Errorf("%s: no extension to %q", testvalue, actual)
		}

		// now check our output
		contents, err := ioutil.ReadFile(actual)
		if err != nil {
			t.Fatal(err)
		}

		fileContents := string(contents)
		for _, plugin := range strings.Split(testvalue, ",") {
			if !strings.Contains(fileContents, strings.ToLower(plugin)) {
				t.Errorf("%s: unable to find %s in output file: %q", testvalue, plugin, fileContents)
			}
		}
	}
}
