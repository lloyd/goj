package goj_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lloyd/goj"
)

func TestParserOffsetParse(t *testing.T) {
	var tcs parseTestCases
	if err := filepath.Walk("testdata/cases", func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(p) != ".json" {
			// Ignore non json files as we're going to read only their information.
			return nil
		}

		gc, err := os.ReadFile(p + ".gold.offset")
		if err != nil {
			// If we can't find the results file we're also going to ignore.
			return nil
		}

		jc, err := os.ReadFile(p)
		if err != nil {
			// For some reason we were not able to open the existing test file. Fail the tests.
			return err
		}

		name := path.Base(p)
		tcs = append(tcs, &parseTestCase{
			name: name[:len(name)-5], // Remove .json from the base name.
			json: string(jc),
			gold: string(gc),
		})
		return nil
	}); err != nil {
		t.Fatal("Error parsing test case data", err)
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			parser := goj.NewParser()
			if err := parser.OffsetParse([]byte(tc.json), func(what goj.Type, key []byte, start, end int) goj.Action {
				if len(key) > 0 {
					got += fmt.Sprintf("key: '%s'\n", string(key))
				}
				got += fmt.Sprintf("%s: %d %d\n", what, start, end)
				return goj.Continue
			}); err != nil {
				got += fmt.Sprintf("parse error: %s\n", err)
			}

			got = strings.TrimRight(got, "\n")
			want := strings.TrimRight(tc.gold, "\n")
			if got != want {
				t.Errorf("Validation failed.\n\nGot:\n%s\n\nWant:\n%s", got, want)
				printJSON(t, []byte(tc.json))
			}
		})
	}
}

func printJSON(t *testing.T, jd []byte) {
	t.Helper()
	for i := range jd {
		t.Logf("[%d]: %c\n", i, jd[i])
	}
}
