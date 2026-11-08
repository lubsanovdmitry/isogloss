// Package render turns simulation state into readable output: JSON for the
// UI and dumps, etymology traces, comparative tables and CLDF exports. It
// uses no reflection (keeping the WASM bundle small).
package render

import (
	"strconv"
	"unicode/utf8"
)

// JSON is a minimal streaming JSON writer.
type JSON struct {
	buf   []byte
	comma []bool // per open container: whether the next value needs a comma
	key   bool   // a key was just written
}

func (j *JSON) pre() {
	if j.key {
		j.key = false
		return
	}
	if n := len(j.comma); n > 0 {
		if j.comma[n-1] {
			j.buf = append(j.buf, ',')
		}
		j.comma[n-1] = true
	}
}

// Obj opens an object.
func (j *JSON) Obj() *JSON {
	j.pre()
	j.buf = append(j.buf, '{')
	j.comma = append(j.comma, false)
	return j
}

// Arr opens an array.
func (j *JSON) Arr() *JSON {
	j.pre()
	j.buf = append(j.buf, '[')
	j.comma = append(j.comma, false)
	return j
}

// End closes the innermost object.
func (j *JSON) End() *JSON {
	j.comma = j.comma[:len(j.comma)-1]
	j.buf = append(j.buf, '}')
	return j
}

// EndArr closes the innermost array.
func (j *JSON) EndArr() *JSON {
	j.comma = j.comma[:len(j.comma)-1]
	j.buf = append(j.buf, ']')
	return j
}

// K writes an object key.
func (j *JSON) K(k string) *JSON {
	j.pre()
	j.buf = appendStr(j.buf, k)
	j.buf = append(j.buf, ':')
	j.key = true
	return j
}

// S writes a string value.
func (j *JSON) S(s string) *JSON {
	j.pre()
	j.buf = appendStr(j.buf, s)
	return j
}

// I writes an integer value.
func (j *JSON) I(n int64) *JSON {
	j.pre()
	j.buf = strconv.AppendInt(j.buf, n, 10)
	return j
}

// U writes an unsigned integer value.
func (j *JSON) U(n uint64) *JSON {
	j.pre()
	j.buf = strconv.AppendUint(j.buf, n, 10)
	return j
}

// B writes a boolean.
func (j *JSON) B(b bool) *JSON {
	j.pre()
	j.buf = strconv.AppendBool(j.buf, b)
	return j
}

// Null writes null.
func (j *JSON) Null() *JSON {
	j.pre()
	j.buf = append(j.buf, "null"...)
	return j
}

// Raw writes an already-encoded JSON value.
func (j *JSON) Raw(b []byte) *JSON {
	j.pre()
	j.buf = append(j.buf, b...)
	return j
}

// KS, KI, KU, KB write a key with a value.
func (j *JSON) KS(k, v string) *JSON        { return j.K(k).S(v) }
func (j *JSON) KI(k string, v int64) *JSON  { return j.K(k).I(v) }
func (j *JSON) KU(k string, v uint64) *JSON { return j.K(k).U(v) }
func (j *JSON) KB(k string, v bool) *JSON   { return j.K(k).B(v) }

// Bytes returns the encoded JSON.
func (j *JSON) Bytes() []byte { return j.buf }

// String returns the encoded JSON.
func (j *JSON) String() string { return string(j.buf) }

const hexDigits = "0123456789abcdef"

func appendStr(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b = append(b, '\\', c)
			case c == '\n':
				b = append(b, '\\', 'n')
			case c == '\t':
				b = append(b, '\\', 't')
			case c < 0x20:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&15])
			default:
				b = append(b, c)
			}
			i++
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		b = append(b, s[i:i+n]...)
		i += n
	}
	return append(b, '"')
}
