package goj

import (
	"errors"
	"fmt"
)

type Type uint8

const (
	Bool Type = iota
	String
	Number
	True
	False
	Null
	Array
	Object
	End
)

func (t Type) String() string {
	switch t {
	case Bool:
		return "bool"
	case String:
		return "string"
	case Number:
		return "number"
	case True:
		return "true"
	case False:
		return "false"
	case Null:
		return "null"
	case Array:
		return "array"
	case Object:
		return "object"
	case End:
		return "end"
	}
	return "<unknown>"
}

type state uint8

const (
	sValue state = iota
	sValueEnd
	sObject
	sArray
	sEnd
)

type Callback func(Type, []byte, []byte) bool

type Parser struct {
	buf      []byte
	i        int
	keystack [][]byte
	states   []state
	s        state
	cb       Callback
}

func (p *Parser) end() bool {
	return p.i >= len(p.buf)
}

// XXX: rewrite in ASM
func (p *Parser) skipSpace() {
	offset := p.i
outer:
	for len(p.buf) > offset {
		switch p.buf[offset] {
		case '\t', '\n', ' ':
			offset++
		default:
			break outer
		}
	}
	p.i = offset
}

// XXX: rewrite in ASM
func (p *Parser) skipNum() {
	offset := p.i
outer:
	for len(p.buf) > offset {
		switch p.buf[offset] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			offset++
		default:
			break outer
		}
	}
	p.i = offset
}

// XXX: rewrite in ASM
func (p *Parser) skipStringContent() {
	offset := p.i
skipping:
	for len(p.buf) > offset {
		switch p.buf[offset] {
		case '\\':
			offset += 2
		case '"':
			break skipping
		default:
			offset++
		}
	}
	p.i = offset
}

func (p *Parser) isNum() bool {
	switch p.buf[p.i] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}

// XXX: how do we handle end of buffer
func (p *Parser) readString() ([]byte, error) {
	if p.buf[p.i] != '"' {
		return nil, p.pError("string expected '\"'")
	}
	p.i++
	start := p.i
	p.skipStringContent()
	if len(p.buf) > p.i && p.buf[p.i] != '"' {
		return nil, p.pError("closing '\"' expected")
	}
	bs := p.buf[start:p.i]
	p.i++
	return bs, nil
}

func (p *Parser) readNumber() ([]byte, error) {
	start := p.i

	if len(p.buf) > p.i {
		switch p.buf[p.i] {
		case '-':
			p.i++
			if len(p.buf) <= p.i || !p.isNum() {
				return nil, p.pError("number expected")
			}
			p.skipNum()
		case '0':
			p.i++
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			p.skipNum()
		}
		if p.i == start {
			return nil, p.pError("number expected")
		}
		if len(p.buf) > p.i && p.buf[p.i] == '.' {
			p.i++
			if !p.isNum() {
				return nil, p.pError("digit expected after decimal point")
			}
			p.skipNum()
		}

		// now handle scentific notation suffix
		// XXX
	}
	return p.buf[start:p.i], nil
}

func (p *Parser) pError(es string) error {
	err := string(p.buf[p.i:])
	if len(err) > 20 {
		err = err[0:20] + "..."
	}
	es += fmt.Sprintf(" at '%s' (%v)", err, p.s)
	return errors.New(es)
}

func (p *Parser) pushState(ns state) {
	p.states = append(p.states, ns)
	p.s = ns
}

func (p *Parser) popState() {
	if len(p.states) > 0 {
		p.s = p.states[len(p.states)-1]
		p.states = p.states[:len(p.states)-1]
	} else {
		p.s = sEnd
	}
}

func (p *Parser) restoreState() {
	if len(p.states) > 0 {
		p.s = p.states[len(p.states)-1]
	} else {
		p.s = sEnd
	}
}

func (p *Parser) send(t Type, v []byte) {
	var k []byte
	if len(p.states) > 0 && p.states[len(p.states)-1] == sObject {
		k = p.keystack[len(p.keystack)-1]
		p.keystack = p.keystack[:len(p.keystack)-1]
	}
	p.cb(t, k, v)
}

func NewParser() *Parser {
	return &Parser{
		nil,
		0,
		make([][]byte, 0, 4),
		make([]state, 0, 4),
		sValue,
		nil,
	}
}

func (p *Parser) Parse(buf []byte, cb Callback) error {
	p.buf = buf
	p.i = 0
	p.s = sValue
	p.keystack = p.keystack[:0]
	p.states = p.states[:0]
	p.cb = cb
	depth := 0

scan:
	for len(p.buf) > p.i {
		switch p.s {
		case sValueEnd:
			if len(p.states) == 0 {
				break scan
			} else {
				switch p.states[len(p.states)-1] {
				case sObject:
					p.skipSpace()
					if len(p.buf) <= p.i {
						return p.pError("premature end")
					} else if p.buf[p.i] == ',' {
						p.s = sObject
					} else if p.buf[p.i] == '}' {
						p.popState()
						p.s = sValueEnd
						p.cb(End, nil, nil)
					} else {
						return p.pError("1 unexpected character")
					}
					p.i++
				case sArray:
					p.skipSpace()
					if len(p.buf) <= p.i {
						return p.pError("premature end")
					} else if p.buf[p.i] == ',' {
						p.s = sValue
					} else if p.buf[p.i] == ']' {
						p.popState()
						p.s = sValueEnd
						p.cb(End, nil, nil)
					} else {
						return p.pError("2 unexpected character")
					}
					p.i++
				default:
					panic("internal inconsistency")
				}
			}
		case sValue:
			// eat whitespace
			p.skipSpace()
			if len(p.buf) <= p.i {
				return p.pError("unexpected end of buffer")
			}
			switch p.buf[p.i] {
			case '{':
				depth++
				p.i++
				p.send(Object, nil)
				p.pushState(sObject)
			case '[':
				depth++
				p.i++
				p.send(Array, nil)
				p.pushState(sArray)
				// skip straight to parsing the first value
				p.s = sValue
			case '"':
				if v, err := p.readString(); err != nil {
					return err
				} else {
					// now we've got a string we've read. wtf to do
					p.restoreState()
					p.send(String, v)
					p.s = sValueEnd
				}
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				if v, err := p.readNumber(); err != nil {
					return err
				} else {
					p.restoreState()
					p.send(Number, v)
					p.s = sValueEnd
				}
			case 'n':
				if len("null") < len(buf)-p.i && buf[p.i+1] == 'u' && buf[p.i+2] == 'l' && buf[p.i+3] == 'l' {
					p.i += len("null")
					p.restoreState()
					p.send(Null, nil)
					p.s = sValueEnd
				} else {
					return p.pError("unexpected character")
				}
			case 't':
				if len("true") < len(buf)-p.i && buf[p.i+1] == 'r' && buf[p.i+2] == 'u' && buf[p.i+3] == 'e' {
					p.i += len("true")
					p.restoreState()
					p.send(True, nil)
					p.s = sValueEnd
				} else {
					return p.pError("unexpected character")
				}
			case 'f':
				if len("false") < len(buf)-p.i && buf[p.i+1] == 'a' && buf[p.i+2] == 'l' && buf[p.i+3] == 's' && buf[p.i+4] == 'e' {
					p.i += len("false")
					p.restoreState()
					p.send(False, nil)
					p.s = sValueEnd
				} else {
					return p.pError("unexpected character")
				}
			default:
				return p.pError("3 unexpected character")
			}
		case sObject:
			p.skipSpace()
			if len(p.buf) <= p.i {
				return p.pError("premature end")
			} else if p.buf[p.i] == '}' {
				p.popState()
				p.s = sValueEnd
			} else if k, err := p.readString(); err != nil {
				return err
			} else {
				p.skipSpace()
				if len(p.buf) <= p.i || p.buf[p.i] != ':' {
					return p.pError("expected ':' to separate key and value")
				}
				p.i++
				//stash k, and enter value state
				p.keystack = append(p.keystack, k)
				p.s = sValue
			}
		default:
			return p.pError(fmt.Sprintf("hit unimplemented state: %s", p.s))
		}
	}

	return nil
}
