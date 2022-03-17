//go:build amd64
// +build amd64

package goj

// ASM optimized scanning routines
func hasAsm() bool
func scanNumberCharsASM(s []byte, offset int) int
func scanNonSpecialStringCharsASM(s []byte, offset int) int

func scanBraces(s []byte, offset int) int
func scanBrackets(s []byte, offset int) int

func (p *Parser) skipObject() error {
	return p.skipSection(scanBraces, '{', '}')
}

func (p *Parser) skipArray() error {
	return p.skipSection(scanBrackets, '[', ']')
}

func (p *Parser) checkBounds(buf []byte) {
	if recordNearPage(buf) {
		p.scanNonSpecialStringChars = scanNonSpecialStringCharsGo
		p.scanNumberChars = scanNumberCharsGo
	} else {
		p.scanNonSpecialStringChars = scanNonSpecialStringCharsASM
		p.scanNumberChars = scanNumberCharsASM
	}
}
