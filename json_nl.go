package goj

import (
	"bufio"
	"io"
)

const bufSize = 4194304 // 4meg

// ReadJSONNL - Read and parse newline separated JSON from an `io.Reader`
// invoke callback with each token.  Terminate if callback returns false.
// arguments to callback:
//   t - token type
//   key - key if parsing object key / value pairs
//   value - decoded value
//   line - line offset in file.  Distinct documents are indicated by a distinct line number.
func ReadJSONNL(s io.Reader, cb func(t Type, key []byte, value []byte, line int64) bool) error {
	reader := bufio.NewReaderSize(s, bufSize)
	var lineNumber int64
	var err error
	var line []byte
	parser := NewParser()
	for line, err = reader.ReadSlice('\n'); err == nil; line, err = reader.ReadSlice('\n') {
		err := parser.Parse(line, func(t Type, k []byte, v []byte) Action {
			if cb(t, k, v, lineNumber) {
				return Continue
			}
			return Cancel
		})
		if err != nil {
			break
		}
		lineNumber++
	}
	// If we reached end of file and the line contents are empty, don't return an additional line.
	if err == io.EOF {
		err = nil
		if len(line) > 0 {
			err = parser.Parse(line, func(t Type, k []byte, v []byte) Action {
				if cb(t, k, v, lineNumber) {
					return Continue
				}
				return Cancel
			})
		}
	} else {
		err = parser.Parse(line, func(t Type, k []byte, v []byte) Action {
			if cb(t, k, v, lineNumber) {
				return Continue
			}
			return Cancel
		})
	}
	return err
}
