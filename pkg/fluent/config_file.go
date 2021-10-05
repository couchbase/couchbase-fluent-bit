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

package fluent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	standardOutputPlugin = "stdout"
)

// Check if this is just the existing default output.
func isDefaultOutput(output string) bool {
	return strings.ToLower(strings.TrimSpace(output)) == standardOutputPlugin
}

// Work out the list of @include lines to add to the file.
func getExtraLinesToAdd(enableOutputPlugins, configDir string) ([]string, error) {
	lines := make([]string, 0)

	for _, requested := range strings.Split(strings.ToLower(enableOutputPlugins), ",") {
		outputPlugin := strings.TrimSpace(requested)

		// Skip if empty, e.g. finish with a comma
		if outputPlugin == "" {
			continue
		}

		// Skip the stdout plugin as provided by default
		if isDefaultOutput(outputPlugin) {
			continue
		}

		// Construct the full plugin include using the /fluent-bit/etc/couchbase location (or its configured value)
		outputPluginFile := fmt.Sprintf("%s/out-%s.conf", configDir, outputPlugin)

		// Check output plugin exists before adding
		fileInfo, err := os.Stat(outputPluginFile)
		if err != nil {
			return lines, fmt.Errorf("unable to load requested output plugin configuration: %w", err)
		}

		// Skip directories
		if fileInfo.IsDir() {
			continue
		}

		// Now add our plugin
		lines = append(lines, fmt.Sprintf("\n# Requested via annotation for %s", outputPlugin))
		lines = append(lines, fmt.Sprintf("\n@include %s", outputPluginFile))
	}

	return lines, nil
}

// AddDynamicConfig creates a temporary copy of the specified configuration file and adds any extra
// output configuration based on the other parameters. If no extra configuration is required then
// the original filename is returned as no need for a temporary file.
func AddDynamicConfig(configFile, configDir, enableOutputPlugins string) (string, error) {
	linesToAdd, err := getExtraLinesToAdd(enableOutputPlugins, configDir)
	if err != nil {
		return configFile, err
	}

	if len(linesToAdd) == 0 {
		return configFile, nil
	}

	log.Infow("Request to enable extra output", "request", enableOutputPlugins)

	// Copy contents of current file then add the extra includes
	contents, err := ioutil.ReadFile(filepath.Clean(configFile))
	if err != nil {
		return configFile, fmt.Errorf("unable to open file %q: %w", configFile, err)
	}

	// Copy file to temporary
	tmpfile, err := ioutil.TempFile("", "fluent-bit-*.conf")
	if err != nil {
		return configFile, fmt.Errorf("unable to create temporary config file: %w", err)
	}
	defer tmpfile.Close()

	_, err = tmpfile.Write(contents)
	if err != nil {
		return configFile, fmt.Errorf("unable to write content to config file: %w", err)
	}

	// Add each line to the file
	for _, line := range linesToAdd {
		_, err := tmpfile.WriteString(line)
		if err != nil {
			return configFile, fmt.Errorf("unable to write line to config file: %w", err)
		}
	}

	log.Infow("Created custom config file", "new", tmpfile.Name(), "original", configFile)

	return tmpfile.Name(), nil
}
