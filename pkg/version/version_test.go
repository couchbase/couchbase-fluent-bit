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

package version_test

import (
	"testing"

	"github.com/couchbase/fluent-bit/pkg/logging"
	"github.com/couchbase/fluent-bit/pkg/version"
)

var (
	log = logging.Log
)

func TestGetVersion(t *testing.T) {
	t.Parallel()
	log.Infow("Test version output",
		"version", version.WithBuildNumber(), "revision", version.GitRevision())

	// confirm it is set correctly based on the test values we hardcode in the makefile
	if expectedVersion := "1-2 (build 3)"; version.WithBuildNumber() != expectedVersion {
		t.Errorf("build number mismatch: %q != %q", version.WithBuildNumber(), expectedVersion)
	}

	if expectedRevision := "456"; version.GitRevision() != expectedRevision {
		t.Errorf("git revision mismatch: %q != %q", version.GitRevision(), expectedRevision)
	}
}
