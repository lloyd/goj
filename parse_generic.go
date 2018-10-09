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
	for x = offset; x < len(s); x++ {
		if s[x] == '"' || s[x] == '\\' || s[x] < 0x20 {
			return x - offset
		}
	}
	return x - offset
}

func scanNumberChars(s []byte, offset int) int {
	x := 0
	for x = offset; x < len(s); x++ {
		if s[x] < '0' || s[x] > '9' {
			return x - offset
		}
	}
	return x - offset
}
