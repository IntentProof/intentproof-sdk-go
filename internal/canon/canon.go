package canon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Marshal returns the RFC 8785 (JCS) canonical JSON encoding of v.
//
// v may be any value that encoding/json can marshal into a JSON value
// composed of objects, arrays, strings, numbers, booleans, and null.
// Numbers that are NaN or ±Inf cause an error to be returned.
//
// Marshal first encodes v with encoding/json (using json.Number for
// numeric parsing to preserve full precision) and then re-emits the
// resulting JSON tree in canonical form. Callers that already have
// pre-marshaled JSON bytes should use MarshalRaw instead.
func Marshal(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canon: marshal input: %w", err)
	}
	return MarshalRaw(raw)
}

// MarshalRaw returns the RFC 8785 (JCS) canonical JSON encoding of
// the JSON value contained in raw.
//
// raw MUST be a complete, well-formed JSON value. Trailing or
// embedded whitespace, comments, or extra tokens cause an error.
func MarshalRaw(raw json.RawMessage) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := decodeValue(dec)
	if err != nil {
		return nil, fmt.Errorf("canon: decode input: %w", err)
	}
	// Reject trailing tokens and malformed suffixes.
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("canon: unexpected trailing JSON token")
	} else if err != io.EOF {
		return nil, fmt.Errorf("canon: malformed trailing data: %w", err)
	}
	var buf bytes.Buffer
	if err := encodeValue(&buf, value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeValue recursively decodes a single JSON value from dec into
// the canonical intermediate representation:
//
//	object  -> *orderedObject (preserves nothing; keys re-sorted later)
//	array   -> []any
//	string  -> string
//	number  -> json.Number
//	bool    -> bool
//	null    -> nil
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := newOrderedObject()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("canon: object key is not a string: %v", keyTok)
				}
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				if err := obj.set(key, val); err != nil {
					return nil, err
				}
			}
			// consume closing '}'
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			var arr []any
			for dec.More() {
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			// consume closing ']'
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			if arr == nil {
				arr = []any{}
			}
			return arr, nil
		default:
			return nil, fmt.Errorf("canon: unexpected delimiter %q", t)
		}
	case string:
		return t, nil
	case json.Number:
		return t, nil
	case bool:
		return t, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("canon: unexpected token type %T", tok)
	}
}

// orderedObject collects keys as they are decoded. They are sorted
// at emission time using UTF-16 code unit ordering.
type orderedObject struct {
	keys   []string
	values map[string]any
}

func newOrderedObject() *orderedObject {
	return &orderedObject{values: map[string]any{}}
}

func (o *orderedObject) set(k string, v any) error {
	if _, exists := o.values[k]; exists {
		return fmt.Errorf("canon: duplicate object key: %s", k)
	}
	o.keys = append(o.keys, k)
	o.values[k] = v
	return nil
}

// encodeValue writes the canonical form of value to buf.
func encodeValue(buf *bytes.Buffer, value any) error {
	switch v := value.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case string:
		return encodeString(buf, v)
	case json.Number:
		return encodeNumber(buf, v)
	case []any:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case *orderedObject:
		return encodeObject(buf, v)
	default:
		return fmt.Errorf("canon: unsupported value type %T", value)
	}
}

// encodeObject sorts the keys of obj by UTF-16 code unit order per
// RFC 8785 §3.2.3 and emits them with their canonical values.
func encodeObject(buf *bytes.Buffer, obj *orderedObject) error {
	keys := make([]string, len(obj.keys))
	copy(keys, obj.keys)
	sort.SliceStable(keys, func(i, j int) bool {
		return lessUTF16(keys[i], keys[j])
	})
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, k); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := encodeValue(buf, obj.values[k]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

// lessUTF16 reports whether a sorts before b when both are compared
// as sequences of UTF-16 code units. This differs from byte-wise
// sorting for characters outside the BMP and for characters whose
// UTF-8 byte order disagrees with their UTF-16 code unit order
// (e.g. U+007F vs supplementary planes, and high vs low surrogate
// halves of supplementary characters).
func lessUTF16(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ar, asz := utf8.DecodeRuneInString(a[ai:])
		br, bsz := utf8.DecodeRuneInString(b[bi:])
		// Compare in UTF-16 code unit space.
		aHi, aLo, aTwo := utf16Units(ar)
		bHi, bLo, bTwo := utf16Units(br)
		if aHi != bHi {
			return aHi < bHi
		}
		// If first units equal, compare second units. Strings without
		// a surrogate pair have a "second unit" of -1 which is less
		// than any real unit; that is the correct ordering since the
		// shorter UTF-16 sequence sorts first.
		var aNext, bNext int32
		if aTwo {
			aNext = int32(aLo)
		} else {
			aNext = -1
		}
		if bTwo {
			bNext = int32(bLo)
		} else {
			bNext = -1
		}
		if aNext != bNext {
			return aNext < bNext
		}
		ai += asz
		bi += bsz
	}
	return len(a)-ai < len(b)-bi
}

// utf16Units returns the UTF-16 code units of r. For BMP code points
// it returns (r, 0, false). For supplementary planes it returns
// (high, low, true).
func utf16Units(r rune) (hi, lo uint16, two bool) {
	if r < 0x10000 {
		return uint16(r), 0, false
	}
	h, l := utf16.EncodeRune(r)
	return uint16(h), uint16(l), true
}

// encodeString writes s as a JSON string using the RFC 8785 §3.2.2.2
// minimal-escape form.
func encodeString(buf *bytes.Buffer, s string) error {
	buf.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("canon: invalid UTF-8 in string at byte %d", i)
		}
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				// Other C0 control characters: \u00XX with lowercase hex.
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteString(s[i : i+size])
			}
		}
		i += size
	}
	buf.WriteByte('"')
	return nil
}

// encodeNumber writes n in the ES6 Number.prototype.toString form
// required by RFC 8785 §3.2.2.3.
func encodeNumber(buf *bytes.Buffer, n json.Number) error {
	s := n.String()
	if s == "" {
		return errors.New("canon: empty number")
	}
	// Try as int64 first to preserve exact integer formatting when
	// possible (avoids floating-point reformatting like "1.0" -> "1").
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		// In ES6, all numbers are IEEE-754 doubles. Integers in the
		// safe range still round-trip exactly; outside it, defer to
		// the float path so the canonical form matches what a JS
		// implementation would produce.
		if i >= -(1<<53)+1 && i <= (1<<53)-1 {
			buf.WriteString(strconv.FormatInt(i, 10))
			return nil
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("canon: invalid number %q: %w", s, err)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("canon: non-finite number %q is not representable", s)
	}
	buf.WriteString(formatES6(f))
	return nil
}

// formatES6 formats f using the ECMAScript Number.prototype.toString
// algorithm. The result is the shortest decimal string that, when
// parsed back as a double, yields the same value, formatted with the
// presentation rules from ECMA-262 §7.1.12.1 (k, n, s decomposition).
//
// This implementation builds on strconv.FormatFloat with 'e'
// formatting and -1 precision, which is documented to produce the
// minimum number of digits required to represent the value uniquely
// (the same property ES6 requires).
func formatES6(f float64) string {
	if f == 0 {
		// Both +0 and -0 canonicalize to "0" per ES6 ToString.
		return "0"
	}
	negative := f < 0
	if negative {
		f = -f
	}
	// Use 'e' with -1 precision to get shortest scientific form.
	// strconv guarantees this is round-trip stable for float64.
	s := strconv.FormatFloat(f, 'e', -1, 64)
	// Parse the "<digits>[.<digits>]e<sign><exp>" form.
	eIdx := strings.IndexByte(s, 'e')
	mantissa := s[:eIdx]
	expStr := s[eIdx+1:]
	exp, err := strconv.Atoi(expStr)
	if err != nil {
		// Should be unreachable; fall back to FormatFloat default.
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	// Split mantissa into the digit sequence k digits long.
	var digits string
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		digits = mantissa[:dot] + mantissa[dot+1:]
	} else {
		digits = mantissa
	}
	// Strip trailing zeros from digits (preserve at least one digit),
	// adjusting exp to keep the value unchanged. strconv's -1 prec
	// output should already be shortest, so this is normally a no-op,
	// but defensive in case of an edge case like "1.0e+00".
	for len(digits) > 1 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
	}
	k := len(digits)
	// In ES6 ToString terms, the value equals digits * 10^(exp-k+1).
	// Define n such that the value is digits * 10^(n-k):
	n := exp + 1
	// ECMA-262 §7.1.12.1 cases:
	//   - If k <= n <= 21: digits followed by (n-k) zeros, no '.'.
	//   - If 0 < n <= 21: first n digits, '.', remaining digits.
	//   - If -6 < n <= 0: "0." + (-n) zeros + digits.
	//   - Else: scientific form.
	var out string
	switch {
	case k <= n && n <= 21:
		out = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		out = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		out = "0." + strings.Repeat("0", -n) + digits
	default:
		// Scientific. Use 'e+<exp>' or 'e-<exp>' with no leading zero
		// on the exponent. The mantissa is digits[0] or
		// digits[0] '.' digits[1:].
		var mant string
		if k == 1 {
			mant = digits
		} else {
			mant = digits[:1] + "." + digits[1:]
		}
		es := n - 1
		var sign string
		if es >= 0 {
			sign = "+"
		} else {
			sign = "-"
			es = -es
		}
		out = mant + "e" + sign + strconv.Itoa(es)
	}
	if negative {
		out = "-" + out
	}
	return out
}
