package goj

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const maxTestBufSize = 17317

func TestScanNumbers(t *testing.T) {
	buf := make([]byte, maxTestBufSize)
	for i := 0; i < maxTestBufSize; i++ {
		buf[i] = byte('0' + (i % 10))
	}

	for i := 0; i < maxTestBufSize; i++ {
		x := scanNumberChars(buf[i:], 0)
		assert.Equal(t, maxTestBufSize, i+x)
	}
}

func TestScanPastEnd(t *testing.T) {
	// 32 byte buffer
	buf := make([]byte, 32)
	// polulated with alpha data
	for i := 0; i < len(buf); i++ {
		buf[i] = byte('a' + (i % 26))
	}

	// but there's a quote at byte 8
	buf[8] = '"'

	// given a slice from bytes 2..4,
	// we shouldn't detect this quote
	slice := buf[2:4]
	slicelen := len(slice)
	x := scanNonSpecialStringChars(slice, 0)
	assert.Equal(t, slicelen, x)
}

func TestScanNonSpecialStringChars(t *testing.T) {
	buf := make([]byte, maxTestBufSize)
	for i := 0; i < maxTestBufSize; i++ {
		buf[i] = byte('a' + (i % 26))
	}
	assert.Equal(t, maxTestBufSize, scanNonSpecialStringChars(buf, 0))

	for i := 0; i < maxTestBufSize; i++ {
		x := scanNonSpecialStringChars(buf, i)
		assert.Equal(t, maxTestBufSize, i+x)
	}
}
