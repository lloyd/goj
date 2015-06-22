package test

import (
	"testing"

	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

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

func getTests() []*parseTestCase {
	tests := make(map[string]*parseTestCase)

	pathToTestCases := "./cases"

	filepath.Walk(pathToTestCases, func(p string, info os.FileInfo, err error) error {
		// skip '^.', '~$', and non-files
		if strings.HasPrefix(path.Base(p), ".") || strings.HasSuffix(p, "~") || strings.HasSuffix(p, "#") || info.IsDir() {
			return nil
		}

		// get the extension
		ext := path.Ext(p)

		// only allow .json, .gold
		if ext != ".json" && ext != ".gold" {
			fmt.Println("WARNING: unsupported suffix, ignoring file:", p)
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
		contents = []byte(strings.TrimSpace(string(contents)))

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
	err := parser.Parse([]byte(json), func(t goj.Type, k []byte, v []byte) bool {
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
		case goj.End:
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if last {
				results += "array close ']'\n"
			} else {
				results += "map close '}'\n"
			}
		}
		//fmt.Printf("got %s [%s] -> '%s'\n", t, string(k), string(v))
		return true
	})
	if err != nil {
		results += fmt.Sprintf("parse error: %s\n", err)
	}

	return results
}

func TestParser(t *testing.T) {
	cases := getTests()

	for _, c := range cases {
		results := testParse(c.json)

		if results != c.gold {
			errText := ""
			for _, s := range strings.Split(c.gold, "\n") {
				errText += "- " + s + "\n"
			}
			for _, s := range strings.Split(results, "\n") {
				errText += "+ " + s + "\n"
			}
			t.Errorf("%s:\n%s\n", c.name, errText)
		}
	}
}
