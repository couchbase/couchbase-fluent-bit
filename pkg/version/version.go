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

package version

import (
	"fmt"
)

var (
	version     string
	revision    string
	buildNumber string
	gitRevision = ""
)

// WithRevision will generate things like 1.0.0 and 1.0.0-beta1 and should be used
// for things like docker images.
func WithRevision() string {
	v := version

	if revision != "" {
		v = v + "-" + revision
	}

	return v
}

// WithBuildNumber will generate things like "1.0.0 (build 123)" and should be used for
// binary version strings so we can tell exactly which build (and by extension
// commit) is being used.
func WithBuildNumber() string {
	return fmt.Sprintf("%s (build %s)", WithRevision(), buildNumber)
}

// GitRevision returns the SCM revision information for development builds to
// pinpoint the exact source code tree a defect was raised against.  If the
// information is not available we default to an official release.
func GitRevision() string {
	if gitRevision == "" {
		gitRevision = "release"
	}

	return gitRevision
}
