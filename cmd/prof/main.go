package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/lloyd/goj"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	fname := fmt.Sprintf("goj_%d.prof", os.Getpid())
	f, err := os.Create(fname)
	if err != nil {
		fmt.Printf("fatal: %s\n", err)
		return
	}

	fmt.Printf("profiling goj with output to %s\n", fname)

	start := time.Now()
	numRead := 0

	pprof.StartCPUProfile(f)
	parser := goj.NewParser()
	InPlaceReadLine(os.Stdin, func(line []byte, lineNum int64, offset int64) error {
		err := parser.Parse(line, func(t goj.Type, k []byte, v []byte) goj.Action {
			return goj.Continue
		})

		if err != nil {
			fmt.Printf("Parse error: %s\n", err)
			return err
		}

		numRead++
		return nil
	})
	pprof.StopCPUProfile()

	fmt.Printf("%d read in %s\n", numRead, time.Since(start).String())
}

const bufSize = 4194304 // 4meg

// InPlaceReadLine invokes the provided callback for every line read.
func InPlaceReadLine(s io.Reader, cb func([]byte, int64, int64) error) error {
	reader := bufio.NewReaderSize(s, bufSize)
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
