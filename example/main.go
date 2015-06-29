// a simple json scanner which reads newline separated json and plucks out and
// prints values under the '.name' property

package main

import (
	"fmt"
	"os"

	"github.com/lloyd/goj"
)

func main() {
	var lastLine int64
	lastLine = 0
	depth := 0
	err := goj.ReadJSONNL(os.Stdin, func(t goj.Type, key []byte, value []byte, line int64) bool {
		if line > lastLine {
			fmt.Printf("\n") // no name key in the last object
			lastLine = line
		}
		switch t {
		case goj.String:
			if depth == 1 && string(key) == "name" {
				fmt.Printf("%s\n", string(value))
				lastLine++
			}
		case goj.Array, goj.Object:
			depth++
		case goj.ArrayEnd, goj.ObjectEnd:
			depth--
		}
		return true
	})
	if err != nil {
		fmt.Printf("Parse error: %s\n", err)
	}
}
