package goj_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lloyd/goj"
)

type parseTestCase struct {
	name string
	json string
	gold string
}

type parseTestCases []*parseTestCase

func (jtc parseTestCases) Len() int {
	return len(jtc)
}

func (jtc parseTestCases) Less(i, j int) bool {
	return jtc[i].name < jtc[j].name
}

func (jtc parseTestCases) Swap(i, j int) {
	jtc[i], jtc[j] = jtc[j], jtc[i]
}

func getTests(t *testing.T) []*parseTestCase {
	t.Helper()

	tests := make(map[string]*parseTestCase)
	pathToTestCases := "testdata/cases"
	filepath.Walk(pathToTestCases, func(p string, info os.FileInfo, err error) error {
		// skip '^.', '~$', and non-files
		if strings.HasPrefix(path.Base(p), ".") || strings.HasSuffix(p, "~") || strings.HasSuffix(p, "#") || info.IsDir() {
			return nil
		}

		// get the extension
		ext := path.Ext(p)

		// only allow .json, .gold
		if ext != ".json" && ext != ".gold" {
			t.Log("WARNING: unsupported suffix, ignoring file:", p)
			return nil
		}
		// relativize and hack off ext to get name
		name := p[0 : len(p)-len(ext)]

		if path.Ext(name) == ".json" {
			name = name[0 : len(name)-len(path.Ext(name))]
		}

		name, _ = filepath.Rel(pathToTestCases, name)

		c, ok := tests[name]
		if !ok {
			c = &parseTestCase{name: name}
			tests[name] = c
		}

		// read the whole file
		contents, err := ioutil.ReadFile(p)
		if err != nil {
			panic(fmt.Sprintf("Couldn't read %s: %s", p, err))
		}

		if ext == ".json" {
			c.json = string(contents)
		} else if ext == ".gold" {
			c.gold = string(contents)
		} else {
			panic("internal inconsistency in parse tests")
		}
		return nil
	})

	// pack into a slice, and error check as we do
	qSlice := make([]*parseTestCase, 0, len(tests))
	for _, v := range tests {
		if v.json == "" {
			panic(fmt.Sprintf("case %s missing json", v.name))
		}

		if v.gold == "" {
			panic(fmt.Sprintf(".gold file missing for case: %s", v.name))
		}

		qSlice = append(qSlice, v)
	}

	// sort slice for deterministic execution order
	sort.Sort(parseTestCases(qSlice))

	return qSlice
}

func testParse(json string) (results string) {
	var stack []bool
	parser := goj.NewParser()
	err := parser.Parse([]byte(json), func(t goj.Type, k []byte, v []byte) goj.Action {
		if len(k) > 0 {
			results += fmt.Sprintf("key: '%s'\n", string(k))
		}
		switch t {
		case goj.True:
			results += "bool: true\n"
		case goj.False:
			results += "bool: false\n"
		case goj.Null:
			results += "null\n"
		case goj.String:
			results += fmt.Sprintf("string: '%s'\n", v)
		case goj.Array:
			results += "array open '['\n"
			stack = append(stack, true)
		case goj.Object:
			results += "map open '{'\n"
			stack = append(stack, false)
		case goj.Integer, goj.NegInteger, goj.Float:
			results += fmt.Sprintf("%s: %s\n", t.String(), v)
		case goj.ArrayEnd:
			stack = stack[:len(stack)-1]
			results += "array close ']'\n"
		case goj.ObjectEnd:
			stack = stack[:len(stack)-1]
			results += "map close '}'\n"
		}
		return goj.Continue
	})
	if err != nil {
		results += fmt.Sprintf("parse error: %s\n", err)
	}

	return results
}

func TestParser(t *testing.T) {
	cases := getTests(t)

	for _, c := range cases {
		results := testParse(c.json)

		want := strings.TrimRight(c.gold, "\n")
		got := strings.TrimRight(results, "\n")
		if got != want {
			errText := ""
			for _, s := range strings.Split(want, "\n") {
				errText += "- " + s + "\n"
			}
			for _, s := range strings.Split(got, "\n") {
				errText += "+ " + s + "\n"
			}
			t.Errorf("%s:\n%s\n", c.name, errText)
		}
	}
}
