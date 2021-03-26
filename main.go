package main

// This is a version of the Kubesphere fork to add config watching with additional Couchbase specific functionality.
// https://github.com/kubesphere/fluent-bit
// As such we try to keep the Kubesphere components as-is to make it easier to update from their repo later.
// We add a watcher for rebalance reports as these require some custom pre-processing to satisfy tail input needs.

import (
	"context"
	"flag"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
)

const (
	binPath      = "/fluent-bit/bin/fluent-bit"
	cfgPath      = "/fluent-bit/etc/fluent-bit.conf"
	watchDir     = "/fluent-bit/config"
	MaxDelayTime = time.Minute * 5
	ResetTime    = time.Minute * 10
)

var (
	logger       log.Logger
	cmd          *exec.Cmd
	mutex        sync.Mutex
	restartTimes int
	timer        *time.Timer
)

// Inspired by https://github.com/jimmidyson/configmap-reload
func isValidEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func start() {
	mutex.Lock()
	defer mutex.Unlock()

	if cmd != nil {
		return
	}

	cmd = exec.Command(binPath, "-c", cfgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = level.Error(logger).Log("msg", "start Fluent bit error", "error", err)
		cmd = nil

		return
	}

	_ = level.Info(logger).Log("msg", "Fluent bit started")
}

func wait() {
	if cmd == nil {
		return
	}

	startTime := time.Now()
	_ = level.Error(logger).Log("msg", "Fluent bit exited", "error", cmd.Wait())
	// Once the fluent bit has executed for 10 minutes without any problems,
	// it should resets the restart backoff timer.
	if time.Since(startTime) >= ResetTime {
		restartTimes = 0
	}

	mutex.Lock()
	cmd = nil
	mutex.Unlock()
}

func backoff() {
	delayTime := time.Duration(math.Pow(2, float64(restartTimes))) * time.Second
	if delayTime >= MaxDelayTime {
		delayTime = MaxDelayTime
	}

	timer.Reset(delayTime)

	startTime := time.Now()

	<-timer.C

	_ = level.Info(logger).Log("msg", "delay", "actual", time.Since(startTime), "expected", delayTime)

	restartTimes++
}

func stop() {
	mutex.Lock()
	defer mutex.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := cmd.Process.Kill(); err != nil {
		_ = level.Info(logger).Log("msg", "Kill Fluent Bit error", "error", err)
	} else {
		_ = level.Info(logger).Log("msg", "Killed Fluent Bit")
	}
}

func resetTimer() {
	if timer != nil {
		if !timer.Stop() {
			<-timer.C
		}

		timer.Reset(0)
	}

	restartTimes = 0
}

// Everything above is direct from Kubesphere.
const (
	maxCBFiles = 5
)

// Be nicer to use lumberjack or similar with a logger just to rotate and remove older files.
func removeOldestFiles(rebalanceOutputDir string) error {
	_ = level.Debug(logger).Log("msg", "Checking for older files to remove", "dir", rebalanceOutputDir)

	files, err := ioutil.ReadDir(rebalanceOutputDir)
	if err != nil {
		return err
	}

	// Skip directories that may be present by just overwriting them in-place
	i := 0

	for _, f := range files {
		if f.IsDir() {
			_ = level.Warn(logger).Log("msg", "Nested directory so skipping removal", "dir", f.Name())
			continue
		}

		// Copy over current entry to next output location
		files[0] = f
		// Increment output location
		i++
	}

	// Now clip off the extras at the end
	files = files[:i]

	if len(files) > maxCBFiles {
		_ = level.Info(logger).Log("msg", "Too many files so removing older files", "dir", rebalanceOutputDir, "count", len(files), "max", maxCBFiles)

		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime().Before(files[j].ModTime())
		})

		// Now we have them from oldest to most recent, remove the oldest up to the max
		filesToDelete := files[:len(files)-maxCBFiles]
		_ = level.Debug(logger).Log("msg", "Removing "+strconv.Itoa(len(filesToDelete))+" files")

		for _, f := range filesToDelete {
			_ = level.Debug(logger).Log("msg", "Removing old file", "file", f.Name())

			err = os.Remove(rebalanceOutputDir + f.Name())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func processFile(filename, rebalanceOutputDir string) error {
	_ = level.Info(logger).Log("msg", "Processing file "+filename)

	// Read the contents in one go, we could do it section by section for better memory usage later
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	// Copy file to temporary
	tmpfile, err := ioutil.TempFile(rebalanceOutputDir, "rebalance-processed-*.json")
	if err != nil {
		return err
	}
	defer tmpfile.Close()

	_ = level.Info(logger).Log("msg", "Creating file "+tmpfile.Name(), "original", filename)

	// Default to current time
	originalTimestamp := time.Now().Format(time.RFC3339)

	// Extract the time we ran the original from the name
	re := regexp.MustCompile(`.*rebalance_report_(?P<time>.*)\.json`)

	match := re.FindStringSubmatch(filename)
	if len(match) > 1 {
		originalTimestamp = match[1]
	}

	// It would be nicer just to use a JSON logger here
	_, err = tmpfile.WriteString(`{"timestamp":"` + originalTimestamp + `", "reportName":"` + filename + `", "reportContents":`)
	if err != nil {
		return err
	}

	_, err = tmpfile.Write(contents)
	if err != nil {
		return err
	}

	_, err = tmpfile.WriteString("}\n")
	if err != nil {
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}

	// Once we have created a file, remove the oldest if more than maxCBFiles
	return removeOldestFiles(rebalanceOutputDir)
}

func processExisting(couchbaseWatchDir, rebalanceOutputDir string) error {
	// Deal with any existing files
	files, err := ioutil.ReadDir(couchbaseWatchDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		filename := couchbaseWatchDir + f.Name()

		err = processFile(filename, rebalanceOutputDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// Disable cyclometric complexity check as the intention is to align this with the Kubesphere version
// so we can easily update if needs be.
//nolint:golint,cyclop
func createWatchers(couchbaseWatchDir, rebalanceOutputDir string) (*run.Group, error) {
	// Based on the KubeSphere version
	var g run.Group
	{
		// Termination handler - this is so if you kill it explicitly it doesn't keep restarting.
		g.Add(run.SignalHandler(context.Background(), os.Interrupt, syscall.SIGTERM))
	}
	{
		// This is Couchbase-specific

		// Watch for new rebalance log files, copy them and add a new line plus timestamp
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			_ = level.Error(logger).Log("msg", "Unable to create file watcher", "error", err)
			return nil, err
		}

		err = watcher.Add(couchbaseWatchDir)
		if err != nil {
			_ = level.Error(logger).Log("msg", "Unable to watch directory", "dir", couchbaseWatchDir, "error", err)
			return nil, err
		}

		done := make(chan bool)
		g.Add(
			func() error {
				for {
					select {
					case <-done:
						return nil
					case event := <-watcher.Events:
						if !isValidEvent(event) {
							continue
						}
						// Now we need to get the filename and copy it to the actual tailed location
						// The mount should be read-only and we do not want to edit-in-place anyway so take a temporary copy to work with
						filename := couchbaseWatchDir + event.Name
						err := processFile(filename, rebalanceOutputDir)
						if err != nil {
							_ = level.Error(logger).Log("msg", "Error reading file", "file", filename, "error", err)
						}

						// watch for errors
					case err := <-watcher.Errors:
						_ = level.Error(logger).Log("msg", "Watcher error", err)
						return nil
					}
				}
			},
			func(err error) {
				_ = watcher.Close()
				close(done)
			},
		)
	}
	{
		// Watch the Fluent bit, if the Fluent bit not exists or stopped, restart it.
		cancel := make(chan struct{})
		g.Add(
			func() error {
				for {
					select {
					case <-cancel:
						return nil
					default:
					}

					// Start fluent bit if it does not existed.
					start()
					// Wait for the fluent bit exit.
					wait()
					// After the fluent bit exit, fluent bit watcher restarts it with an exponential
					// back-off delay (1s, 2s, 4s, ...), that is capped at five minutes.
					backoff()
				}
			},
			func(err error) {
				close(cancel)
				stop()
				resetTimer()
			},
		)
	}
	{
		// Watch the config file, if the config file changed, stop Fluent bit.
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			_ = level.Error(logger).Log("err", err)
			return nil, err
		}

		// Start watcher.
		err = watcher.Add(watchDir)
		if err != nil {
			_ = level.Error(logger).Log("err", err)
			return nil, err
		}

		cancel := make(chan struct{})
		g.Add(
			func() error {
				for {
					select {
					case <-cancel:
						return nil
					case event := <-watcher.Events:
						if !isValidEvent(event) {
							continue
						}

						// After the config file changed, it should stop the fluent bit,
						// and resets the restart backoff timer.
						stop()
						resetTimer()
						_ = level.Info(logger).Log("msg", "Config file changed, stop Fluent Bit")
					case <-watcher.Errors:
						_ = level.Error(logger).Log("msg", "Watcher stopped")
						return nil
					}
				}
			},
			func(err error) {
				_ = watcher.Close()
				close(cancel)
			},
		)
	}

	return &g, nil
}

func getDirectory(defaultValue, environmentVariable string) string {
	directoryName := os.Getenv(environmentVariable)
	if directoryName == "" {
		_ = level.Info(logger).Log("msg", "No environment variable so defaulting", "environmentVariable", environmentVariable, "defaultValue", defaultValue)
		directoryName = defaultValue
	}

	directoryName = path.Clean(directoryName)

	// Make sure we end with a slash
	if !strings.HasSuffix(directoryName, "/") {
		directoryName += "/"
	}

	return directoryName
}

func main() {
	ignoreExisting := flag.Bool("ignoreExisting", true, "Ignore any existing rebalance reports, if false will process then exit")
	createWatchedDirectory := flag.Bool("createWatchedDirectory", true, "Auto-create the directories if they do not exist that we watch")
	flag.Parse()

	logger = log.NewLogfmtLogger(os.Stdout)
	_ = level.Info(logger).Log("msg", "Starting up CB-FB processor", "ignoreExisting", *ignoreExisting)

	couchbaseWatchDir := getDirectory("/opt/couchbase/var/couchbase/logs", "COUCHBASE_LOGS") + "rebalance/"
	_ = level.Info(logger).Log("msg", "Watching for rebalance reports in: "+couchbaseWatchDir)

	rebalanceOutputDir := getDirectory("/tmp/rebalance-logs", "COUCHBASE_LOGS_REBALANCE_TEMPDIR")
	_ = level.Info(logger).Log("msg", "Temporary processed rebalance reports in: "+rebalanceOutputDir)

	err := os.Mkdir(rebalanceOutputDir, 0700)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Unable to create rebalance output directory", "dir", rebalanceOutputDir, "error", err)
	}

	if !*ignoreExisting {
		// Deal with any existing ones
		err := processExisting(couchbaseWatchDir, rebalanceOutputDir)
		if err != nil {
			_ = level.Error(logger).Log("msg", "Unable to read files in CB directory", "error", err, "inputDir", couchbaseWatchDir, "outputDir", rebalanceOutputDir)
		}

		_ = level.Info(logger).Log("msg", "Processed all existing ones so exiting")

		os.Exit(0)
	}
	if *createWatchedDirectory {
		err := os.MkdirAll(couchbaseWatchDir, 0755)
		if err != nil {
			_ = level.Error(logger).Log("msg", "Unable to create CB directory", "error", err, "inputDir", couchbaseWatchDir)
		}
	}

	timer = time.NewTimer(0)

	runGroup, err := createWatchers(couchbaseWatchDir, rebalanceOutputDir)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Unable to create watchers", "error", err, "inputDir", couchbaseWatchDir, "outputDir", rebalanceOutputDir)
	} else {
		err := runGroup.Run()
		if err != nil {
			_ = level.Error(logger).Log("msg", "Unable to run", "error", err)
		}
	}

	_ = level.Info(logger).Log("msg", "Exiting Couchbase Watcher")
}
