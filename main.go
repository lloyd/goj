package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	//	"runtime/pprof"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	fname := fmt.Sprintf("query_%d.prof", os.Getpid())
	_, err := os.Create(fname)
	if err != nil {
		fmt.Printf("fatal: %s\n", err)
		return
	}

	fmt.Printf("profiling query with output to %s\n", fname)

	start := time.Now()
	numRead := 0

	//	pprof.StartCPUProfile(f)
	parser := NewParser()
	InPlaceReadLine(os.Stdin, func(line []byte, lineNum int64, offset int64) error {
		err := parser.Parse(line, func(t Type, k []byte, v []byte) bool {
			switch t {
			case True:
			case False:
			case Null:
			case String:
			case Array:
			case Object:
			case End:
			}
			//fmt.Printf("got %s [%s] -> '%s'\n", t, string(k), string(v))
			return true
		})

		if err != nil {
			fmt.Printf("Parse error: %s\n", err)
			return err
		}

		numRead++
		return nil
	})
	//	pprof.StopCPUProfile()

	fmt.Printf("%d read in %s\n", numRead, time.Since(start).String())
}

const BUF_SIZE = 4194304 // 4meg

// invoke callback for every read line with
func InPlaceReadLine(s io.Reader, cb func([]byte, int64, int64) error) error {
	reader := bufio.NewReaderSize(s, BUF_SIZE)
	var count int64
	var offset int64
	var err error
	var line []byte
	for line, err = reader.ReadSlice('\n'); err == nil; line, err = reader.ReadSlice('\n') {
		if err = cb(line[:len(line)-1], count, offset); err != nil {
			return err
		}
		offset += int64(len(line))
		count++
	}
	// If we reached end of file and the line contents are empty, don't return an additional line.
	if err == io.EOF {
		err = nil
		if len(line) > 0 {
			return cb(line, count, offset)
		}
	} else {
		return cb(line, count, offset)
	}
	return nil
}
