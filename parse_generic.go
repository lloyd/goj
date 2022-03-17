//go:build !amd64
// +build !amd64

package goj

func hasAsm() bool {
	return false
}

func (p *Parser) skipObject() error {
	return p.skipSection(scanBracesGo, '{', '}')
}

func (p *Parser) skipArray() error {
	return p.skipSection(scanBracketsGo, '[', ']')
}

func (p *Parser) checkBounds(buf []byte) {
}
