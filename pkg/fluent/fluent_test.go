package fluent_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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
	config := fluent.NewFluentBitConfig("/bin/bash", getCustomCommand(testFile), dir)

	if testFileExists(testFile) {
		t.Error("Test file already exists at the start")
	}

	fluent.Start(config)
	fluent.Wait(config)

	if !testFileExists(testFile) {
		t.Error("Unable to find expected test output file")
	}
}

// Check that we can actually stop the binary.
func TestCommandStop(t *testing.T) {
	t.Parallel()

	// Run forever (well a long time)
	config := fluent.NewFluentBitConfig("/bin/bash", "sleep 10000", "")

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
	config := fluent.NewFluentBitConfig("/bin/bash", "sleep 10000", dir)

	var g run.Group
	if err := fluent.AddDynamicConfigWatcher(&g, config); err != nil {
		t.Fatal(err)
	}

	if testFileExists(testFile) {
		t.Error("Test file already exists at the start")
	}

	if config.GetRestartCount() != 0 {
		t.Errorf("Invalid restart count at the start: %d", config.GetRestartCount())
	}

	{
		g.Add(func() error {
			for i := 0; i < 5; i++ {
				dst, err := os.Create(filepath.Join(dir, filepath.Base("test_file_"+strconv.Itoa(i))))
				if err != nil {
					t.Fatal(err, i)
				}
				// Make sure we close it straight away to flush
				_ = dst.Close()
				// Allow us time to restart Fluent Bit
				time.Sleep(2 * time.Second)
				// Even though it is restarted a few times, it is only once for each change
				if config.GetRestartCount() != 1 {
					t.Errorf("Invalid restart count: %d != %d", config.GetRestartCount(), 1)
				}
			}

			return nil
		}, func(err error) {
			if err != nil {
				t.Errorf("Error during test: %v", err)
			}
		})
	}

	if err := g.Run(); err != nil {
		t.Errorf("Error during test: %v", err)
	}
}
