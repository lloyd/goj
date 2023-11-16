//go:build !amd64
// +build !amd64

package goj

func hasAsm() bool {
	return false
}
func scanNumberCharsASM(s []byte, offset int) int {
	return scanNumberCharsGo(s, offset)
}
func scanNonSpecialStringCharsASM(s []byte, offset int) int {
	return scanNonSpecialStringCharsGo(s, offset)
}

func scanBraces(s []byte, offset int) int {
	return scanBracesGo(s, offset)
}
func scanBrackets(s []byte, offset int) int {
	return scanBracketsGo(s, offset)
}
