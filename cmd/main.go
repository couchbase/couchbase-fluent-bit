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

package main

// This is a version of the Kubesphere fork to add config watching with additional Couchbase specific functionality.
// https://github.com/kubesphere/fluent-bit
// We add a watcher for rebalance reports as these require some custom pre-processing to satisfy tail input needs.

import (
	"flag"
	"os"

	"github.com/couchbase/fluent-bit/pkg/common"
	"github.com/couchbase/fluent-bit/pkg/couchbase"
	"github.com/couchbase/fluent-bit/pkg/logging"
)

var (
	log = logging.Log
)

func main() {
	ignoreExisting := flag.Bool("ignoreExisting", true, "Ignore any existing rebalance reports, if false will process then exit")
	flag.Parse()

	common.LoadEnvironment()
	log.Infow("Starting up Couchbase log processor", "ignoreExisting", *ignoreExisting, "environment", os.Environ())

	config := couchbase.NewWatcherConfigFromDefaults()
	if err := config.CreateRebalanceDir(); err != nil {
		log.Errorw("Issue with creating rebalance output directory", "error", err, "config", config)
	}

	// To simplify integration testing we add a special mode
	// This allows us to just parse existing ones rather than wait for them to appear
	if !*ignoreExisting {
		if err := couchbase.ProcessExisting(*config); err != nil {
			log.Errorw("Unable to process existing files in CB directory", "error", err, "config", config)
		}

		log.Info("Processed all existing ones so exiting")

		os.Exit(0)
	}

	// Set up our watchers and then start running everything
	runGroup, err := couchbase.CreateWatchers(*config)
	if err != nil {
		log.Errorw("Unable to create watchers", "error", err, "config", config)
	} else {
		err := runGroup.Run()
		if err != nil {
			log.Errorw("Unable to run", "error", err)
		}
	}

	log.Info("Exiting Couchbase Watcher")
}
