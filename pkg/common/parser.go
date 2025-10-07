/*
 *  Copyright 2022 Couchbase, Inc.
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

package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type MembufLimitConfig struct {
	NumInputs        int
	NumOutputs       int
	MemBufLimitNames []string
}

type ConfigFileParser struct {
	ConfigFile *[]string
	Inputs     [][]string
	Outputs    [][]string
}

// Looks explicitly at ${UPPER_CASE} env vars
// and extracts UPPER_CASE.
func extractEnvVar(line string) string {
	re := regexp.MustCompile(`\$\{([A-Z_]+)\}`) // check for env variables.
	matches := re.FindAllStringSubmatch(line, -1)

	if len(matches) > 0 && len(matches[0]) > 1 {
		return matches[0][1]
	}

	return ""
}

// findNextPrefix returns the index of the line where the prefix first appears
// -1 if it isn't found.
func findNextPrefix(configFile *[]string, prefix string, searchFrom int) int {
	length := len(*configFile)
	for j := searchFrom; j < length; j++ {
		nextLine := strings.ToUpper(strings.TrimSpace((*configFile)[j]))
		if strings.HasPrefix(nextLine, prefix) {
			return j
		} else if strings.HasPrefix(nextLine, "[") {
			return -1
		}
	}

	return -1
}

// since an input section might be for the audit logs but audit is disabled
// or audit is enabled but there are no inputs for audit.
func handleAuditInputs(startingIndex int, configFile *[]string) int {
	if foundIndex := findNextPrefix(configFile, "PATH", startingIndex+1); foundIndex != -1 {
		if strings.HasSuffix((*configFile)[foundIndex], "AUDIT.LOG") {
			if GetAuditEnabled() {
				return 1
			}

			return 0
		}
	}

	return 1
}

// CreateConfigFileParser returns a basic config parser.
func CreateConfigFileParser(configFile *[]string) *ConfigFileParser {
	parser := &ConfigFileParser{ConfigFile: configFile, Inputs: nil, Outputs: nil}

	return parser
}

// extractSection starts from `sectionStart` and returns a slice from
// sectionStart to the end of file or the next section.
func (p *ConfigFileParser) extractSection(sectionStart int) []string {
	length := len(*p.ConfigFile)

	var section []string
	section = append(section, (*p.ConfigFile)[sectionStart])
	i := sectionStart + 1

	for ; i < length; i++ {
		nextLine := strings.ToUpper(strings.TrimSpace((*p.ConfigFile)[i]))
		if strings.HasPrefix(nextLine, "[") {
			break
		}

		section = append(section, nextLine)
	}

	return section
}

// BuildSection parses the config file into the parsers
// Input and Output properties.
func (p *ConfigFileParser) BuildSections() *ConfigFileParser {
	for i := range *p.ConfigFile {
		line := strings.ToUpper(strings.TrimSpace((*p.ConfigFile)[i]))
		switch line {
		case "[INPUT]":
			p.Inputs = append(p.Inputs, p.extractSection(i))
		case "[OUTPUT]":
			p.Outputs = append(p.Outputs, p.extractSection(i))
		}
	}

	return p
}

// CreateMemBufLimitConfig takes a built configfile and returns
// all mememory buffer limit environment variables and
// total number of enabled inputs and outputs.
func CreateMemBufLimitConfig(configFile *[]string) *MembufLimitConfig {
	config := &MembufLimitConfig{NumOutputs: 0, NumInputs: 0, MemBufLimitNames: nil}
	parser := CreateConfigFileParser(configFile).BuildSections()
	outputCount := 0
	inputCount := 0

	for j, inputSection := range parser.Inputs { // for each inputs check
		for i := range inputSection {
			inputCount += handleAuditInputs(i, &parser.Inputs[j])

			if foundIndex := findNextPrefix(&parser.Inputs[j], "MEM_BUF_LIMIT", i+1); foundIndex != -1 {
				envVar := extractEnvVar(inputSection[foundIndex])
				config.MemBufLimitNames = append(config.MemBufLimitNames, envVar)

				break
			}
		}
	}

	for j, outputSection := range parser.Outputs {
		for i := range outputSection {
			if foundIndex := findNextPrefix(&parser.Outputs[j], "MATCH", i+1); foundIndex != -1 {
				if envValue := os.Getenv(extractEnvVar(outputSection[foundIndex])); envValue != "" && envValue != "no-match" {
					outputCount++

					break
				}
			}
		}
	}

	config.NumInputs = inputCount
	config.NumOutputs = outputCount

	return config
}

// handleIncludeFilePaths takes the directory of the initial config file
// and the file path of file which needs to be imported
// to build a final path.
func handleIncludeFilePaths(baseDir, includeFilePath string) string {
	// swap any env variables for their real path
	if configDirEnvVar := extractEnvVar(includeFilePath); configDirEnvVar != "" {
		if filePath := os.Getenv(configDirEnvVar); filePath != "" {
			includeFilePath = strings.Replace(includeFilePath, fmt.Sprintf("${%s}", configDirEnvVar), filePath, 1)
		}
	}

	// is absolute file so just return it
	if strings.HasPrefix(includeFilePath, "/") {
		return includeFilePath
	}

	// build absolute path
	dir := filepath.Dir(baseDir)

	return filepath.Join(dir, includeFilePath)
}

// BuildConfigFile starts with a root config and follows @includes
// to build a complete config.
func BuildConfigFile(filepath string) (*[]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %w", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	var completeConfig []string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "@include "):
			includeFilePath := strings.TrimPrefix(line, "@include ")
			includeFilePath = handleIncludeFilePaths(filepath, includeFilePath)
			newLines, err := BuildConfigFile(includeFilePath)

			if err != nil {
				return nil, err
			}

			completeConfig = append(completeConfig, *newLines...)
		case strings.HasPrefix(line, "#"):
			continue
		default:
			completeConfig = append(completeConfig, line)
		}
	}

	return &completeConfig, nil
}
