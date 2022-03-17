package goj

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

// Global PageSize variable so a sys call is not made each time
var PageSize = uintptr(os.Getpagesize())

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
	// SkippedData represent the []byte of data that was skipped.
	SkippedData
)

// Action drives the behavior from the callback
type Action uint8

const (
	Continue Action = iota
	// Cancel the parsing
	Cancel
	// Skips the current content and invoke callback when over with the []slice
	Skip
)

//go:nosplit
func scanNonSpecialStringCharsGo(s []byte, offset int) (x int) {
	for i, c := range s[offset:] {
		if c == '"' || c == '\\' || c < 0x20 {
			return i
		}
	}
	return len(s) - offset
}

//go:nosplit
func scanNumberCharsGo(s []byte, offset int) (x int) {
	for i, c := range s[offset:] {
		if c < '0' || c > '9' {
			return i
		}
	}
	return len(s) - offset
}

//go:nosplit
func scanBracesGo(s []byte, offset int) int {
	for i, c := range s[offset:] {
		if c == '{' || c == '}' || c == '"' {
			return i
		}
	}
	return len(s) - offset
}

//go:nosplit
func scanBracketsGo(s []byte, offset int) int {
	for i, c := range s[offset:] {
		if c == '[' || c == ']' || c == '"' {
			return i
		}
	}
	return len(s) - offset
}

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
	case SkippedData:
		return "skipped data"
	}
	return "<unknown>"
}

func (a Action) String() string {
	switch a {
	case Continue:
		return "continue"
	case Cancel:
		return "cancel"
	case Skip:
		return "skip"
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
	sClientSkippingObject // will skip an entire value
	sClientSkippingArray  // will skip an entire value
)

func (s state) isSkipping() bool {
	return s > sClientCancelledParse
}

// Callback is the signature of the client callback to the parsing routine.
// The routine is passed the type of entity parsed, a key if relevant
// (parsing inside an object), and a decoded value.
type Callback func(what Type, key []byte, value []byte) Action

// Parser is the primary object provided by goj via the NewParser method.
// The various parsing routines are provided by this object, but it has no
// exported fields.
type Parser struct {
	buf                       []byte
	i                         int
	keyStack                  [][]byte
	states                    []state
	s                         state
	callback                  Callback
	offsetCallback            OffsetCallback
	cookedBuf                 []byte
	scanNumberChars           func(s []byte, offset int) int
	scanNonSpecialStringChars func(s []byte, offset int) int
}

func (p *Parser) cb(t Type, k, v []byte) {
	ok := p.callback(t, k, v)
	switch ok {
	case Continue:
	case Cancel:
		p.s = sClientCancelledParse
	case Skip:
		if t == Object {
			p.s = sClientSkippingObject
		} else if t == Array {
			p.s = sClientSkippingArray
		}
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

func (p *Parser) readString() ([]byte, bool, error) {
	buf := p.buf
	if buf[p.i] != '"' {
		return nil, false, p.pError("string expected '\"'")
	}
	p.i++
	start := p.i
	offset := p.i
	p.cookedBuf = p.cookedBuf[0:0]

	for len(buf) > offset {
		offset += p.scanNonSpecialStringChars(buf, offset)
		if len(buf) <= offset {
			return nil, false, p.pError("unterminated string found")
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
					return nil, false, p.pError("unexpected EOF after '\\u'")
				}
				r, err := strconv.ParseInt(string(buf[offset:offset+4]), 16, 0)
				if err != nil {
					return nil, false, p.pError("invalid (non-hex) character occurs after '\\u' inside string.")
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
				return nil, false, p.pError("inside a string, '\\' occurs before a character which it may not")
			}
		case '"':
			p.i = offset + 1
			if len(p.cookedBuf) > 0 {
				return append(p.cookedBuf, buf[start:offset]...), true, nil
			}
			return buf[start:offset], false, nil
		default:
			if c >= 0x20 {
				offset++
			} else {
				p.i += offset
				return nil, false, p.pError("invalid character inside string")
			}
		}
	}

	if len(buf) <= offset || buf[offset] != '"' {
		p.cookedBuf = p.cookedBuf[0:0]
		return nil, false, p.pError("unterminated string found")
	}
	p.i = offset + 1
	if len(p.cookedBuf) > 0 {
		b := append(p.cookedBuf, buf[start:offset]...)
		p.cookedBuf = p.cookedBuf[0:0]
		return b, true, nil
	}
	return buf[start:offset], false, nil
}

func (p *Parser) readNumber() ([]byte, Type, error) {
	start, end, t, err := p.readNumberOffset()
	if err != nil {
		return nil, t, err
	}
	return p.buf[start:end], t, nil
}

func (p *Parser) skipString() error {
	buf := p.buf
	if buf[p.i] != '"' {
		return p.pError("string expected '\"'")
	}
	p.i++
	offset := p.i

	for len(buf) > offset {
		offset += p.scanNonSpecialStringChars(buf, offset)
		if len(buf) <= offset {
			return p.pError("unterminated string found")
		}
		c := buf[offset]
		switch c {
		case '\\':
			offset++
			switch buf[offset] {
			case '\\', '/', '"':
				offset++
			case 't':
				offset++
			case 'n':
				offset++
			case 'r':
				offset++
			case 'b':
				offset++
			case 'f':
				offset++
			case 'u':
				offset++
				if len(buf)-offset < 4 {
					return p.pError("unexpected EOF after '\\u'")
				}
				r, err := strconv.ParseInt(string(buf[offset:offset+4]), 16, 0)
				if err != nil {
					return p.pError("invalid (non-hex) character occurs after '\\u' inside string.")
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
				offset += 5 + surrogateSize
			default:
				// bogus escape
				p.i += offset
				return p.pError("inside a string, '\\' occurs before a character which it may not")
			}
		case '"':
			p.i = offset + 1
			return nil
		default:
			if c >= 0x20 {
				offset++
			} else {
				p.i += offset
				return p.pError("invalid character inside string")
			}
		}
	}
	return p.pError("unterminated string found")
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
		keystack := p.keyStack
		off := len(keystack) - 1
		k := keystack[off]
		p.keyStack = keystack[:off]
		p.cb(t, k, v)
	} else {
		p.cb(t, nil, v)
	}
}

func (p *Parser) skipSection(scan func([]byte, int) int, open, close byte) error {
	// we just skipped the '{'
	start := p.i - 1
	p.skipSpace()
	in := 1
	offset := p.i
	buf := p.buf
	for len(buf) > offset && in > 0 {
		offset += scan(buf, offset)
		if buf[offset] == open {
			offset++
			in++
		} else if buf[offset] == close {
			offset++
			in--
		} else if buf[offset] == '"' {
			p.i = offset
			if err := p.skipString(); err != nil {
				return err
			}
			offset = p.i
		}
	}

	p.i = offset
	if p.callback != nil {
		p.cb(SkippedData, nil, buf[start:offset])
	}
	if p.offsetCallback != nil {
		p.ocb(SkippedData, nil, start, offset)
	}
	p.s = sValueEnd
	return nil
}

// NewParser - Allocate a new JSON Scanner that may be re-used.
func NewParser() *Parser {
	return &Parser{
		keyStack:                  make([][]byte, 0, 4),
		states:                    make([]state, 0, 4),
		s:                         sValue,
		scanNumberChars:           scanNumberCharsGo,
		scanNonSpecialStringChars: scanNonSpecialStringCharsGo,
	}
}

// recordNearPage returns true if the end of the record is within 15 bytes of the end of a page
func recordNearPage(buf []byte) bool {
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	return (hdr.Data+uintptr(hdr.Len)-1)&(PageSize-1) >= PageSize-15
}

// Parse parses a complete JSON document. Callback will be invoked once
// for each JSON entity found.
func (p *Parser) Parse(buf []byte, cb Callback) (err error) {
	p.buf = buf
	p.i = 0
	p.s = sValue
	p.keyStack = p.keyStack[:0]
	p.states = p.states[:0]
	p.callback = cb
	depth := 0
	if hasAsm() {
		if recordNearPage(buf) {
			p.scanNonSpecialStringChars = scanNonSpecialStringCharsGo
			p.scanNumberChars = scanNumberCharsGo
		} else {
			p.scanNonSpecialStringChars = scanNonSpecialStringCharsASM
			p.scanNumberChars = scanNumberCharsASM
		}
	} // else we don't have to ever worry about that.
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
				if !p.s.isSkipping() {
					p.pushState(sObject)
				}
			case '[':
				depth++
				p.i++
				p.send(Array, nil)
				if !p.s.isSkipping() {
					p.pushState(sArray)
				}
			case '"':
				var v []byte
				var err error
				if v, _, err = p.readString(); err != nil {
					return err
				}
				p.restoreState()
				p.send(String, v)
				if p.s != sClientCancelledParse {
					p.s = sValueEnd
				}
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				var t Type
				var v []byte
				var err error
				if v, t, err = p.readNumber(); err != nil {
					return err
				}
				p.restoreState()
				p.send(t, v)
				if p.s != sClientCancelledParse {
					p.s = sValueEnd
				}
			case 'n':
				if len("null") <= len(buf)-p.i && buf[p.i+1] == 'u' && buf[p.i+2] == 'l' && buf[p.i+3] == 'l' {
					p.i += len("null")
					p.restoreState()
					p.send(Null, nil)
					if p.s != sClientCancelledParse {
						p.s = sValueEnd
					}
				} else {
					return p.pError("invalid string in json text.")
				}
			case 't':
				if len("true") <= len(buf)-p.i && buf[p.i+1] == 'r' && buf[p.i+2] == 'u' && buf[p.i+3] == 'e' {
					p.i += len("true")
					p.restoreState()
					p.send(True, nil)
					if p.s != sClientCancelledParse {
						p.s = sValueEnd
					}
				} else {
					return p.pError("invalid string in json text.")
				}
			case 'f':
				if len("false") <= len(buf)-p.i && buf[p.i+1] == 'a' && buf[p.i+2] == 'l' && buf[p.i+3] == 's' && buf[p.i+4] == 'e' {
					p.i += len("false")
					p.restoreState()
					p.send(False, nil)
					if p.s != sClientCancelledParse {
						p.s = sValueEnd
					}
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
				var cooked bool
				if k, cooked, err = p.readString(); err != nil {
					return err
				}
				p.skipSpace()
				if len(buf) <= p.i || buf[p.i] != ':' {
					return p.pError("expected ':' to separate key and value")
				}
				p.i++
				// Stash k, and enter value state
				if cooked {
					// if we're parsing a key, and it resides in the cooked buffer
					// (contained an escape), then we must copy it while
					// we parse the value, which may *also* use the cooked buffer.
					//
					// Assumption: cooked keys are rare, don't worry about the
					// copy!
					buf := make([]byte, len(k))
					copy(buf, k)
					k = buf
				}
				p.keyStack = append(p.keyStack, k)
				p.s = sValue
			}
		case sClientCancelledParse:
			return ClientCancelledParse
		case sClientSkippingObject:
			if err = p.skipObject(); err != nil {
				return err
			}
		case sClientSkippingArray:
			if err = p.skipArray(); err != nil {
				return err
			}
		default:
			return p.pError(fmt.Sprintf("hit unimplemented state: %v", p.s))
		}
	}
	p.skipSpace()
	if !p.end() {
		return p.pError("trailing garbage")
	}
	// is the parse complete?
	if len(p.states) > 0 {
		return p.pError("premature EOF")
	}

	return nil
}
