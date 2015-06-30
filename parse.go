package goj

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// Type represents the JSON value type.
type Type uint8

const (
	// String represents a JSON string.
	String Type = iota
	// Integer represents a JSON number known to be a uint.
	Integer
	// NegInteger represents a JSON number known to be an int.
	NegInteger
	// Float represents a JSON number that is neither an int or uint.
	Float
	// True represents the JSON boolean 'true'.
	True
	// False represents the JSON boolean 'false'.
	False
	// Null represents the JSON null value.
	Null
	// Array represents the beginning of a JSON array.
	Array
	// ArrayEnd represents the end of a JSON array.
	ArrayEnd
	// Object represents the beginning of a JSON object.
	Object
	// ObjectEnd represents the end of a JSON object.
	ObjectEnd
)

func hasAsm() bool
func countSlice(s []uint64) int
func findStrRange(r []byte, s []byte) int
func scanNumberChars(s []byte, offset int) int
func scanNonSpecialStringChars(s []byte, offset int) int
func scanWhitespaceChars(s []byte, offset int) int

func (t Type) String() string {
	switch t {
	case String:
		return "string"
	case Integer:
		return "integer"
	case NegInteger:
		return "negative integer"
	case Float:
		return "float"
	case True:
		return "true"
	case False:
		return "false"
	case Null:
		return "null"
	case Array:
		return "array"
	case ArrayEnd:
		return "array end"
	case Object:
		return "object"
	case ObjectEnd:
		return "object end"
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
	sClientCancelledParse
)

// Callback is the signature of the client callback to the parsing routine.
// The routine is passed the type of entity parsed, a key if relevant
// (parsing inside an object), and a decoded value.
type Callback func(what Type, key []byte, value []byte) bool

// Parser is the primary object provided by goj via the NewParser method.
// The various parsing routines are provided by this object, but it has no
// exported fields.
type Parser struct {
	buf       []byte
	i         int
	keystack  [][]byte
	states    []state
	s         state
	_cb       Callback
	cookedBuf []byte
}

func (p *Parser) cb(t Type, k, v []byte) {
	if !p._cb(t, k, v) {
		p.s = sClientCancelledParse
	}
}

func (p *Parser) end() bool {
	return p.i >= len(p.buf)
}

// Note: ASM version has more overhead for terse json documents.
// Heuristic based optimization possible here.
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

func (p *Parser) addToCooked(start, offset int, r rune) {
	er := make([]byte, 5, 5)
	x := utf8.EncodeRune(er, r)
	p.cookedBuf = append(p.cookedBuf, p.buf[start:offset-1]...)
	p.cookedBuf = append(p.cookedBuf, er[:x]...)
}

func (p *Parser) readString() ([]byte, error) {
	buf := p.buf
	if buf[p.i] != '"' {
		return nil, p.pError("string expected '\"'")
	}
	p.i++
	start := p.i
	offset := p.i

skipping:
	for len(buf) > offset {
		offset += scanNonSpecialStringChars(buf, offset)
		if len(buf) <= offset {
			break
		}
		c := buf[offset]
		switch c {
		case '\\':
			offset++
			switch buf[offset] {
			case '\\', '/', '"':
				p.addToCooked(start, offset, rune(buf[offset]))
				offset++
				start = offset
			case 't':
				p.addToCooked(start, offset, '\t')
				offset++
				start = offset
			case 'n':
				p.addToCooked(start, offset, '\n')
				offset++
				start = offset
			case 'r':
				p.addToCooked(start, offset, '\r')
				offset++
				start = offset
			case 'b':
				p.addToCooked(start, offset, '\b')
				offset++
				start = offset
			case 'f':
				p.addToCooked(start, offset, '\f')
				offset++
				start = offset
			case 'u':
				offset++
				if len(buf)-offset < 4 {
					p.cookedBuf = p.cookedBuf[0:0]
					return nil, p.pError("unexpected EOF after '\\u'")
				}
				r, err := strconv.ParseInt(string(buf[offset:offset+4]), 16, 0)
				if err != nil {
					p.cookedBuf = p.cookedBuf[0:0]
					return nil, p.pError("invalid (non-hex) character occurs after '\\u' inside string.")
				}
				offset--
				// is this a utf16 surrogate marker?
				surrogateSize := 0
				if (r & 0xFC00) == 0xD800 {
					// point just past end of first
					toff := offset + 5
					// enough buffer for second utf16 codepoint?
					if len(buf) <= (toff + 6) {
						r = '?' // not enough buffer
					} else if buf[toff] != '\\' || buf[toff+1] != 'u' {
						r = '?' // surrogate marker not followed by codepoint
					} else {
						surrogate, err := strconv.ParseInt(string(buf[toff+2:toff+6]), 16, 0)
						if err != nil {
							r = '?' // invalid hex in second member of pair
						} else {
							surrogateSize = 6
							r = (((r & 0x3F) << 10) | ((((r >> 6) & 0xF) + 1) << 16) | (surrogate & 0x3FF))
						}
					}
				}
				p.addToCooked(start, offset, rune(r))
				offset += 5 + surrogateSize
				start = offset
			default:
				// bogus escape
				p.i += offset
				p.cookedBuf = p.cookedBuf[0:0]
				return nil, p.pError("inside a string, '\\' occurs before a character which it may not")
			}
		case '"':
			break skipping
		default:
			if c >= 0x20 {
				offset++
			} else {
				p.i += offset
				p.cookedBuf = p.cookedBuf[0:0]
				return nil, p.pError("invalid character inside string")
			}
		}
	}

	if len(buf) <= offset || buf[offset] != '"' {
		p.cookedBuf = p.cookedBuf[0:0]
		return nil, p.pError("unterminated string found")
	}
	p.i = offset + 1
	if len(p.cookedBuf) > 0 {
		b := append(p.cookedBuf, buf[start:offset]...)
		p.cookedBuf = p.cookedBuf[0:0]
		return b, nil
	}
	return buf[start:offset], nil
}

func (p *Parser) readNumber() ([]byte, Type, error) {
	start := p.i
	t := Integer

	if len(p.buf) > p.i {
		switch p.buf[p.i] {
		case '-':
			t = NegInteger
			p.i++
			x := scanNumberChars(p.buf, p.i)
			if x == 0 {
				return nil, t, p.pError("malformed number, a digit is required after the minus sign")
			}
			p.i += x
		case '0':
			p.i++
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			p.i += scanNumberChars(p.buf, p.i)
		}
		if p.i == start {
			return nil, t, p.pError("number expected")
		}
		if len(p.buf) > p.i && p.buf[p.i] == '.' {
			t = Float
			p.i++
			x := scanNumberChars(p.buf, p.i)
			if x == 0 {
				return nil, t, p.pError("digit expected after decimal point")
			}
			p.i += x
		}

		// now handle scentific notation suffix
		if len(p.buf) > p.i && (p.buf[p.i] == 'e' || p.buf[p.i] == 'E') {
			t = Float
			p.i++
			if len(p.buf) > p.i && (p.buf[p.i] == '-' || p.buf[p.i] == '+') {
				p.i++
			}
			x := scanNumberChars(p.buf, p.i)
			if x == 0 {
				return nil, t, p.pError("digits expected after exponent marker (e)")
			}
			p.i += x

		}
	}
	return p.buf[start:p.i], t, nil
}

// The Error object is provided by the Parser when an error is encountered.
type Error struct {
	e      string
	buf    []byte
	offset int
}

func (e *Error) Error() string {
	return e.e
}

// Verbose returns a longer version of the error string, along with a limited
// portion of the JSON around which the error occurred.
func (e *Error) Verbose() string {
	if len(e.buf) <= e.offset {
		return e.e
	}
	err := string(e.buf[e.offset:])
	if len(err) > 20 {
		err = err[0:20] + "..."
	}
	return e.e + fmt.Sprintf(" at '%s' (%v)", err, e.e)
}

// Error code returned from .Parse() when callback returns false.
var ClientCancelledParse = &Error{
	e: "client cancelled parse",
}

func (p *Parser) pError(es string) error {
	return &Error{
		e:      es,
		buf:    p.buf,
		offset: p.i,
	}
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
	states := p.states
	slen := len(states)
	if slen > 0 && states[slen-1] == sObject {
		keystack := p.keystack
		off := len(keystack) - 1
		k := keystack[off]
		p.keystack = keystack[:off]
		p.cb(t, k, v)
	} else {
		p.cb(t, nil, v)
	}

}

// NewParser - Allocate a new JSON Scanner that may be re-used.
func NewParser() *Parser {
	return &Parser{
		nil,
		0,
		make([][]byte, 0, 4),
		make([]state, 0, 4),
		sValue,
		nil,
		nil,
	}
}

// Parse parses a complete JSON document. Callback will be invoked once
// for each JSON entity found.
func (p *Parser) Parse(buf []byte, cb Callback) error {
	// HACK/TODO:  loose about 3% of performance to fix crasher.
	// PCMPISTRI seems to be crashing by running past the end of
	// input strings.  Odd that simply ensuring null padding does not address
	// this.
	b := make([]byte, len(buf), len(buf)+16)
	copy(b, buf)
	buf = b

	p.buf = buf
	p.i = 0
	p.s = sValue
	p.keystack = p.keystack[:0]
	p.states = p.states[:0]
	p._cb = cb
	depth := 0

scan:
	for len(buf) > p.i {
		switch p.s {
		case sValueEnd:
			if len(p.states) == 0 {
				break scan
			} else {
				switch p.states[len(p.states)-1] {
				case sObject:
					p.skipSpace()
					if len(buf) <= p.i {
						return p.pError("premature EOF")
					} else if buf[p.i] == ',' {
						p.s = sObject
					} else if buf[p.i] == '}' {
						p.popState()
						p.s = sValueEnd
						p.cb(ObjectEnd, nil, nil)
					} else {
						return p.pError("after key and value, inside map, I expect ',' or '}'")
					}
					p.i++
				case sArray:
					p.skipSpace()
					if len(buf) <= p.i {
						return p.pError("premature EOF")
					} else if buf[p.i] == ',' {
						p.s = sValue
					} else if buf[p.i] == ']' {
						p.popState()
						p.s = sValueEnd
						p.cb(ArrayEnd, nil, nil)
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
			if len(buf) <= p.i {
				return p.pError("unexpected end of buffer")
			}
			switch buf[p.i] {
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
			case '"':
				var v []byte
				var err error
				if v, err = p.readString(); err != nil {
					return err
				}
				p.restoreState()
				p.send(String, v)
				p.s = sValueEnd
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				var t Type
				var v []byte
				var err error
				if v, t, err = p.readNumber(); err != nil {
					return err
				}
				p.restoreState()
				p.send(t, v)
				p.s = sValueEnd
			case 'n':
				if len("null") <= len(buf)-p.i && buf[p.i+1] == 'u' && buf[p.i+2] == 'l' && buf[p.i+3] == 'l' {
					p.i += len("null")
					p.restoreState()
					p.send(Null, nil)
					p.s = sValueEnd
				} else {
					return p.pError("invalid string in json text.")
				}
			case 't':
				if len("true") <= len(buf)-p.i && buf[p.i+1] == 'r' && buf[p.i+2] == 'u' && buf[p.i+3] == 'e' {
					p.i += len("true")
					p.restoreState()
					p.send(True, nil)
					p.s = sValueEnd
				} else {
					return p.pError("invalid string in json text.")
				}
			case 'f':
				if len("false") <= len(buf)-p.i && buf[p.i+1] == 'a' && buf[p.i+2] == 'l' && buf[p.i+3] == 's' && buf[p.i+4] == 'e' {
					p.i += len("false")
					p.restoreState()
					p.send(False, nil)
					p.s = sValueEnd
				} else {
					return p.pError("invalid string in json text.")
				}
			default:
				return p.pError("unallowed token at this point in JSON text")
			}
		case sArray:
			p.skipSpace()
			if len(buf) <= p.i {
				return p.pError("premature EOF")
			} else if buf[p.i] == ']' {
				p.i++
				p.popState()
				p.s = sValueEnd
				p.cb(ArrayEnd, nil, nil)
			} else {
				p.s = sValue
			}
		case sObject:
			p.skipSpace()
			if len(buf) <= p.i {
				return p.pError("premature EOF")
			} else if buf[p.i] == '}' {
				p.i++
				p.popState()
				p.s = sValueEnd
				p.cb(ObjectEnd, nil, nil)
			} else {
				var k []byte
				var err error
				if k, err = p.readString(); err != nil {
					return err
				}
				p.skipSpace()
				if len(buf) <= p.i || buf[p.i] != ':' {
					return p.pError("expected ':' to separate key and value")
				}
				p.i++
				// Stash k, and enter value state
				p.keystack = append(p.keystack, k)
				p.s = sValue
			}
		case sClientCancelledParse:
			return ClientCancelledParse
		default:
			return p.pError(fmt.Sprintf("hit unimplemented state: %v", p.s))
		}
	}
	p.skipSpace()
	if !p.end() {
		return p.pError("trailing garbage")
	}
	return nil
}
