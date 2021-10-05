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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/couchbase/fluent-bit/pkg/common"
	"github.com/couchbase/fluent-bit/pkg/fluent"
	"github.com/couchbase/fluent-bit/pkg/logging"
	"github.com/fsnotify/fsnotify"
	"github.com/oklog/run"
)

const (
	// MaxCBFiles should only be used here and for testing.
	MaxCBFiles = 5
)

var (
	log = logging.Log
	// ErrNoFluentBitConfig indicates when we fail to create a valid configuration.
	ErrNoFluentBitConfig = errors.New("unable to create valid config object for fluent bit watching")
)

func RemoveOldestFiles(rebalanceOutputDir string) error {
	// Be nicer to use lumberjack or similar with a logger just to rotate and remove older files.
	log.Debugw("Checking for older files to remove", "dir", rebalanceOutputDir)

	files, err := ioutil.ReadDir(rebalanceOutputDir)
	if err != nil {
		return fmt.Errorf("unable to read directory %q for old files: %w", rebalanceOutputDir, err)
	}

	// Skip directories that may be present by just overwriting them in-place
	i := 0

	for _, f := range files {
		if f.IsDir() {
			log.Warnw("Nested directory so skipping removal", "dir", f.Name())

			continue
		}

		// Copy over current entry to next output location
		files[0] = f
		// Increment output location
		i++
	}

	// Now clip off the extras at the end
	files = files[:i]

	if len(files) > MaxCBFiles {
		log.Infow("Too many files so removing older files", "dir", rebalanceOutputDir, "count", len(files), "max", MaxCBFiles)

		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime().Before(files[j].ModTime())
		})

		// Now we have them from oldest to most recent, remove the oldest up to the max
		filesToDelete := files[:len(files)-MaxCBFiles]
		log.Debugf("Removing %d files", len(filesToDelete))

		for _, f := range filesToDelete {
			filenameToRemove := filepath.Join(rebalanceOutputDir, f.Name())
			log.Debugw("Removing old file", "file", filenameToRemove)

			err = os.Remove(filenameToRemove)
			if err != nil {
				return fmt.Errorf("unable to remove old file %q: %w", filenameToRemove, err)
			}
		}
	}

	return nil
}

func ProcessFile(filename, rebalanceOutputDir string) error {
	log.Infof("Processing file %q", filename)

	// The filename must include the directory as well
	filename = filepath.Clean(filename)

	// Read the contents in one go, we could do it section by section for better memory usage later
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to open file %q: %w", filename, err)
	}

	// Copy file to temporary
	tmpfile, err := ioutil.TempFile(rebalanceOutputDir, "rebalance-processed-*.json")
	if err != nil {
		return fmt.Errorf("unable to create temporary output file in %q: %w", rebalanceOutputDir, err)
	}
	defer tmpfile.Close()

	log.Infow("Creating file", "new", tmpfile.Name(), "original", filename)

	// Default to current time
	originalTimestamp := time.Now().Format(time.RFC3339)

	// Extract the time we ran the original from the name
	re := regexp.MustCompile(`.*rebalance_report_(?P<time>.*)\.json`)

	match := re.FindStringSubmatch(filename)
	if len(match) > 1 {
		originalTimestamp = match[1]
	}

	headerContents := `{"timestamp":"` + originalTimestamp + `", "reportName":"` + filename + `", "reportContents":`
	// It would be nicer just to use a JSON logger here
	_, err = tmpfile.WriteString(headerContents)
	if err != nil {
		return fmt.Errorf("unable to write header to output file: %w", err)
	}

	_, err = tmpfile.Write(contents)
	if err != nil {
		return fmt.Errorf("unable to write content to output file: %w", err)
	}

	_, err = tmpfile.WriteString("}\n")
	if err != nil {
		return fmt.Errorf("unable to write ending to output file: %w", err)
	}

	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("unable to close output file: %w", err)
	}

	// Once we have created a file, remove the oldest if more than maxCBFiles
	return RemoveOldestFiles(rebalanceOutputDir)
}

func ProcessExisting(config WatcherConfig) error {
	// Deal with any existing files
	couchbaseWatchDir := filepath.Clean(config.couchbaseWatchDir)

	files, err := ioutil.ReadDir(couchbaseWatchDir)
	if err != nil {
		return fmt.Errorf("unable to read input directory %q: %w", couchbaseWatchDir, err)
	}

	rebalanceOutputDir := filepath.Clean(config.rebalanceOutputDir)

	for _, f := range files {
		filename := filepath.Join(couchbaseWatchDir, f.Name())

		err = ProcessFile(filename, rebalanceOutputDir)
		if err != nil {
			return err
		}
	}

	log.Infow("Processed all existing files in watch directory", "dir", couchbaseWatchDir)

	return nil
}

func rebalanceFileHandler(filename string, config WatcherConfig) {
	// Now we need to get the filename and copy it to the actual tailed location
	// The mount should be read-only and we do not want to edit-in-place anyway so take a temporary copy to work with
	err := ProcessFile(filename, config.rebalanceOutputDir)
	if err != nil {
		log.Errorw("Error reading file", "file", filename, "error", err)
	}
}

func rebalanceDirectoryHandler(watcher *fsnotify.Watcher, config WatcherConfig) bool {
	// On each notification check for existence
	couchbaseWatchDir := filepath.Clean(config.couchbaseWatchDir)

	_, err := os.Stat(couchbaseWatchDir)
	if os.IsNotExist(err) {
		log.Debugw("Rebalance report directory still does not exist", "dir", couchbaseWatchDir)

		return false
	}

	// Remove the current watched directory
	couchbaseLogDir := filepath.Clean(config.couchbaseLogDir)

	err = watcher.Remove(couchbaseLogDir)
	if err != nil {
		log.Errorw("Error removing watch on log directory", "dir", config.couchbaseLogDir, "error", err)
	}

	// process all existing
	err = ProcessExisting(config)
	if err != nil {
		log.Errorw("Unable to read files in rebalance directory", "error", err, "config", config)
	}

	// watch for new ones
	err = watcher.Add(couchbaseWatchDir)
	if err != nil {
		log.Errorw("Unable to watch directory", "dir", couchbaseWatchDir, "error", err)
	}

	log.Errorw("Rebalance report directory now exists so watching", "dir", couchbaseWatchDir)

	return true
}

func AddCouchbaseWatcher(g *run.Group, config WatcherConfig) error {
	couchbaseLogDir := filepath.Clean(config.couchbaseLogDir)
	// Watch for new rebalance log files, copy them and add a new line plus timestamp
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("unable to create couchbase watcher: %w", err)
	}

	couchbaseWatchDir := filepath.Join(couchbaseLogDir, "rebalance")
	// The rebalance directory may not exist when the container starts so we wait for it to appear
	foundRebalance := true

	_, err = os.Stat(couchbaseWatchDir)
	if os.IsNotExist(err) {
		log.Infow("Rebalance report directory does not exist", "dir", couchbaseWatchDir)

		foundRebalance = false
		// Watch the main log directory and wait for rebalance to appear
		err = watcher.Add(couchbaseLogDir)
		if err != nil {
			return fmt.Errorf("unable to add %q to couchbase watcher: %w", couchbaseLogDir, err)
		}
	} else {
		err = watcher.Add(couchbaseWatchDir)
		if err != nil {
			return fmt.Errorf("unable to add %q to couchbase watcher: %w", couchbaseWatchDir, err)
		}
	}

	done := make(chan bool)

	g.Add(
		func() error {
			for {
				select {
				case <-done:
					return nil
				case event := <-watcher.Events:
					if !common.IsValidEvent(event) {
						continue
					}
					log.Debugw("Couchbase watcher event triggered", "event", event)
					if foundRebalance {
						rebalanceFileHandler(event.Name, config)
					} else {
						foundRebalance = rebalanceDirectoryHandler(watcher, config)
					}
				case err := <-watcher.Errors:
					log.Errorw("Couchbase watcher error", "error", err)

					return nil
				}
			}
		},
		func(err error) {
			_ = watcher.Close()
			close(done)
		},
	)

	return nil
}

func CreateWatchers(cw WatcherConfig) (*run.Group, error) {
	fb := fluent.NewFluentBitConfig(
		cw.GetFluentBitBinaryPath(),
		cw.GetFluentBitConfigFilePath(),
		cw.GetWatchedFluentBitConfigDir(),
		cw.GetCouchbaseFluentBitConfigDir(),
		cw.GetExtraOutputPlugins(),
	)
	if fb == nil {
		return nil, ErrNoFluentBitConfig
	}

	// Based on the KubeSphere version
	var g run.Group

	// Termination handler - this is so if you kill it explicitly it doesn't keep restarting.
	g.Add(run.SignalHandler(context.Background(), os.Interrupt, syscall.SIGTERM))

	err := AddCouchbaseWatcher(&g, cw)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to add couchbase watcher", err)
	}

	err = fluent.AddDynamicConfigWatcher(&g, fb)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to add fluent config watcher", err)
	}

	return &g, nil
}
