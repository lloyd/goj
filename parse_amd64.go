//go:build amd64
// +build amd64

package goj

// ASM optimized scanning routines stubs
func hasAsm() bool
func scanNumberCharsASM(s []byte, offset int) int
func scanNonSpecialStringCharsASM(s []byte, offset int) int
func scanBraces(s []byte, offset int) int
func scanBrackets(s []byte, offset int) int
