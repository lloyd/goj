package goj

import (
	"fmt"
)

// OffsetCallback is the signature of the client callback to the offset
// parsing routine.
//
// The routine is passed the type of entity parsed, a key if relevant
// (parsing inside an object), and the start and end offset of the value.
//
// The start offset is inclusive and the end offset is exclusive, which allows
// for callers to simply re-slice their buffers with the received data without
// any manipulation.
//
// There are corner cases for opening and closing of arrays and objects, on
// those scenarios the caller will receive the start value and -1 for end and
// -1 and end value respectively.
type OffsetCallback func(what Type, key []byte, start, end int) Action

// OffsetParse implements lazy parsing that allows for callers to decide how to
// read data from the byte slices.
//
// The callbacks will receive the indices of the raw data without any parsing,
// so the caller is responsible for any decoding if needed. The only exception
// for this is when it's a key object, in that case the parser will decode data
// before calling the callback.
func (p *Parser) OffsetParse(buf []byte, cb OffsetCallback) error {
	p.buf = buf
	p.i = 0
	p.s = sValue
	p.keyStack = p.keyStack[:0]
	p.states = p.states[:0]
	p.offsetCallback = cb
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
						p.ocb(ObjectEnd, nil, -1, p.i+1)
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
						p.ocb(ArrayEnd, nil, -1, p.i+1)
					} else {
						return p.pError("2 unexpected character")
					}
					p.i++
				default:
					return p.pError("internal inconsistency")
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
				p.offsetSend(Object, p.i-1, -1)
				if !p.s.isSkipping() {
					p.pushState(sObject)
				}
			case '[':
				depth++
				p.i++
				p.offsetSend(Array, p.i-1, -1)
				if !p.s.isSkipping() {
					p.pushState(sArray)
				}
			case '"':
				var s, e int
				var err error
				if s, e, err = p.readStringOffset(); err != nil {
					return err
				}
				p.restoreState()
				p.offsetSend(String, s, e)
				if p.s != sClientCancelledParse {
					p.s = sValueEnd
				}
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				var t Type
				var s, e int
				var err error
				if s, e, t, err = p.readNumberOffset(); err != nil {
					return err
				}
				p.restoreState()
				p.offsetSend(t, s, e)
				if p.s != sClientCancelledParse {
					p.s = sValueEnd
				}
			case 'n':
				nullLen := len("null")
				if nullLen <= len(buf)-p.i && buf[p.i+1] == 'u' && buf[p.i+2] == 'l' && buf[p.i+3] == 'l' {
					p.i += nullLen
					p.restoreState()
					p.offsetSend(Null, p.i-nullLen, p.i)
					if p.s != sClientCancelledParse {
						p.s = sValueEnd
					}
				} else {
					return p.pError("invalid string in json text.")
				}
			case 't':
				trueLen := len("true")
				if trueLen <= len(buf)-p.i && buf[p.i+1] == 'r' && buf[p.i+2] == 'u' && buf[p.i+3] == 'e' {
					p.i += trueLen
					p.restoreState()
					p.offsetSend(True, p.i-trueLen, p.i)
					if p.s != sClientCancelledParse {
						p.s = sValueEnd
					}
				} else {
					return p.pError("invalid string in json text.")
				}
			case 'f':
				falseLen := len("false")
				if falseLen <= len(buf)-p.i && buf[p.i+1] == 'a' && buf[p.i+2] == 'l' && buf[p.i+3] == 's' && buf[p.i+4] == 'e' {
					p.i += falseLen
					p.restoreState()
					p.offsetSend(False, p.i-falseLen, p.i)
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
				p.ocb(ArrayEnd, nil, -1, p.i) // FIXME: We're at a value end, so the end offset is p.i. What's the start?
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
				p.ocb(ObjectEnd, nil, -1, p.i)
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
			if err := p.skipObject(); err != nil {
				return err
			}
		case sClientSkippingArray:
			if err := p.skipArray(); err != nil {
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

func (p *Parser) ocb(t Type, k []byte, s, e int) {
	ok := p.offsetCallback(t, k, s, e)
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

func (p *Parser) offsetSend(t Type, s, e int) {
	states := p.states
	sLen := len(states)
	if sLen > 0 && states[sLen-1] == sObject {
		keyStack := p.keyStack
		off := len(keyStack) - 1
		k := keyStack[off]
		p.keyStack = keyStack[:off]
		p.ocb(t, k, s, e)
	} else {
		p.ocb(t, nil, s, e)
	}
}

func (p *Parser) readStringOffset() (int, int, error) {
	start := p.i
	if err := p.skipString(); err != nil {
		return -1, -1, err
	}
	end := p.i
	return start, end, nil
}

func (p *Parser) readNumberOffset() (int, int, Type, error) {
	start := p.i
	t := Integer

	if len(p.buf) > p.i {
		switch p.buf[p.i] {
		case '-':
			t = NegInteger
			p.i++
			x := p.scanNumberChars(p.buf, p.i)
			if x == 0 {
				return -1, -1, t, p.pError("malformed number, a digit is required after the minus sign")
			}
			p.i += x
		case '0':
			p.i++
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			p.i += p.scanNumberChars(p.buf, p.i)
		}
		if p.i == start {
			return -1, -1, t, p.pError("number expected")
		}
		if len(p.buf) > p.i && p.buf[p.i] == '.' {
			t = Float
			p.i++
			x := p.scanNumberChars(p.buf, p.i)
			if x == 0 {
				return -1, -1, t, p.pError("digit expected after decimal point")
			}
			p.i += x
		}

		// now handle scientific notation suffix
		if len(p.buf) > p.i && (p.buf[p.i] == 'e' || p.buf[p.i] == 'E') {
			t = Float
			p.i++
			if len(p.buf) > p.i && (p.buf[p.i] == '-' || p.buf[p.i] == '+') {
				p.i++
			}
			x := p.scanNumberChars(p.buf, p.i)
			if x == 0 {
				return -1, -1, t, p.pError("digits expected after exponent marker (e)")
			}
			p.i += x

		}
	}
	return start, p.i, t, nil
}
