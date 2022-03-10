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

import (
	"bufio"
	"flag"
	"os"
	"regexp"

	jd "github.com/josephburnett/jd/lib"
)

func Readln(r *bufio.Reader) (string, error) {
	var (
		isPrefix       = true
		err      error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return processString(string(ln)), err
}

func readLines(filename string) ([]string, error) {
	f, err := os.Open(filename)

	if err != nil {
		return nil, err
	}

	defer f.Close()
	var lines []string
	r := bufio.NewReader(f)

	s, e := Readln(r)
	for e == nil {
		lines = append(lines, s)
		s, e = Readln(r)
	}
	return lines, nil
}

// processString massages the output logs into JSON
// they're almost valid JSON except for the filename and
// when we __IGNORE__ or __REDACT__ information.
func processString(line string) string {
	newLine := "{ " + line + " }"
	// replace filename with "filename"
	objKeysRegex := regexp.MustCompile(`([{])(\s*)([_/.A-Za-z0-9_\-]+?)\s*:`)
	newLine = objKeysRegex.ReplaceAllString(newLine, "$1\"$3\":")

	// for our __IGNORE__ or __REDACT__
	ignoredPattern := regexp.MustCompile(`(?P<ignore>__[A-Za-z]*__)`)
	newLine = ignoredPattern.ReplaceAllString(newLine, `"$ignore"`)
	return newLine
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 2 {
		panic("Not enough args")
	}

	expected, err := readLines(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	actual, err := readLines(flag.Arg(1))
	if err != nil {
		panic(err)
	}

	if len(expected) != len(actual) {
		panic("Files have different number of lines")
	}
	var diffs jd.Diff

	for i := 0; i < len(expected); i++ {
		a, err := jd.ReadJsonString(expected[i])
		if err != nil {
			panic(err)

		}
		b, err := jd.ReadJsonString(actual[i])
		if err != nil {
			panic(err)
		}
		diff := a.Diff(b)
		if len(diff) != 0 {
			diffs = append(diffs, diff...)
		}
	}
	if len(diffs) != 0 {
		os.Exit(1)
	}

	os.Exit(0)
}
