// +build !amd64

package goj

func hasAsm() bool {
	return false
}

// non-asm routines that perform direct byte scanning.
// bytes.IndexFunc is slower and performs needless coallescing of utf8
// bytes into runes

func scanNonSpecialStringChars(s []byte, offset int) int {
	x := 0
	for x = 0; x < len(s)-offset; x++ {
		r := rune(s[offset+x])
		if r == '"' || r == '\\' || r < 0x20 {
			return x
		}
	}
	return x
}

func scanNumberChars(s []byte, offset int) int {
	x := 0
	for x = 0; x < len(s)-offset; x++ {
		r := rune(s[offset+x])
		if r < '0' || r > '9' {
			return x
		}
	}
	return x
}
