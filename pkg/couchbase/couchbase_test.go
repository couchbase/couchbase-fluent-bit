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

package couchbase_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/couchbase/fluent-bit/pkg/couchbase"
)

func createTestFilesByTimestamp(t *testing.T, dir string) {
	t.Helper()

	// Create a set of numbered files with an incrementing time so we remove the lowest numbers first
	for i := 1; i < couchbase.MaxCBFiles*2; i++ {
		dst, err := os.Create(filepath.Join(dir, filepath.Base("test_file_"+strconv.Itoa(i))))
		if err != nil {
			t.Fatal(err, i)
		}

		defer dst.Close()
		time.Sleep(time.Second)
	}
}

func countFilesInDirectory(t *testing.T, dir string) int {
	t.Helper()

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err, dir)
	}

	return len(files)
}

func createRebalanceTestDir(t *testing.T, baseDir, testName string) string {
	t.Helper()

	dir, err := os.MkdirTemp(baseDir, testName)
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestRemoveOldestFiles(t *testing.T) {
	t.Parallel()

	dir := createRebalanceTestDir(t, "", "oldest_files_test")
	defer os.RemoveAll(dir)

	createTestFilesByTimestamp(t, dir)
	// We now have a set of files ordered by time
	if countFilesInDirectory(t, dir) <= couchbase.MaxCBFiles {
		t.Errorf("Invalid number of files: %d <= %d", countFilesInDirectory(t, dir), couchbase.MaxCBFiles)
	}

	if err := couchbase.RemoveOldestFiles(dir); err != nil {
		t.Fatal(err, dir)
	}

	if countFilesInDirectory(t, dir) > couchbase.MaxCBFiles {
		t.Errorf("Invalid number of files: %d > %d", countFilesInDirectory(t, dir), couchbase.MaxCBFiles)
	}
}
func TestRemoveOldestFilesWithNestedDirs(t *testing.T) {
	t.Parallel()

	dir := createRebalanceTestDir(t, "", "oldest_files_test_with_dir")
	defer os.RemoveAll(dir)
	// Ensure directories are skipped by adding a load to the directory first then test files
	for i := 1; i < couchbase.MaxCBFiles; i++ {
		nestedDir := createRebalanceTestDir(t, dir, "oldest_files_test_with_dir")
		defer os.RemoveAll(nestedDir)
	}

	createTestFilesByTimestamp(t, dir)

	if err := couchbase.RemoveOldestFiles(dir); err != nil {
		t.Fatal(err, dir)
	}

	if countFilesInDirectory(t, dir) <= couchbase.MaxCBFiles {
		t.Errorf("Invalid number of files: %d <= %d", countFilesInDirectory(t, dir), couchbase.MaxCBFiles)
	}
}

func TestProcessFile(t *testing.T) {
	t.Parallel()

	dir := createRebalanceTestDir(t, "", "process_files_test")
	defer os.RemoveAll(dir)

	// Iterate over each test file to check
	inputDir := filepath.Clean("../../test/logs/rebalance")

	files, err := os.ReadDir(inputDir)
	if err != nil {
		t.Fatal(err, inputDir)
	}

	for _, testfile := range files {
		filename := filepath.Join(inputDir, testfile.Name())

		if err := couchbase.ProcessFile(filename, dir); err != nil {
			t.Fatal(err, testfile)
		}
	}

	outputFiles, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err, dir)
	}

	if len(files) != len(outputFiles) {
		t.Error("Mismatch in input vs output", len(files), len(outputFiles))
	}
}
func TestProcessExisting(t *testing.T) {
	t.Parallel()

	// This is mostly as above but using the input directories directly
	dir := createRebalanceTestDir(t, "", "process_existing_test")
	defer os.RemoveAll(dir)

	// Iterate over each test file to check
	inputDir := filepath.Clean("../../test/logs/rebalance")

	config := couchbase.WatcherConfig{}
	config.SetRebalanceOutputDir(dir)
	config.SetCouchbaseWatchDir(inputDir)

	if err := couchbase.ProcessExisting(config); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWatchers(t *testing.T) {
	t.Parallel()

	couchbaseLogDir := "../../test/logs"

	rebalanceOutputDir := createRebalanceTestDir(t, "", "create_watchers_test")
	defer os.RemoveAll(rebalanceOutputDir)

	fluentBitConfigDir := createRebalanceTestDir(t, "", "fluent-bit-config")
	defer os.RemoveAll(fluentBitConfigDir)

	config := couchbase.WatcherConfig{}
	config.SetFluentBitConfigDir(fluentBitConfigDir)
	config.SetCouchbaseLogDir(couchbaseLogDir)
	config.SetRebalanceOutputDir(rebalanceOutputDir)

	if _, err := couchbase.CreateWatchers(config); err != nil {
		t.Fatal(err)
	}
}
