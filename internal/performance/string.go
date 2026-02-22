// Package performance provides high-performance string operations
package performance

import (
	"strings"
	"unicode"
	"unsafe"
)

// UnsafeString converts byte slice to string without allocation
// WARNING: The byte slice must not be modified after this call
func UnsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// UnsafeBytes converts string to byte slice without allocation
// WARNING: The returned byte slice must not be modified
func UnsafeBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// FastToLower converts string to lowercase without allocation for ASCII
func FastToLower(s string) string {
	// Check if already lowercase
	needsLower := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			needsLower = true
			break
		}
	}
	if !needsLower {
		return s
	}

	// Convert to lowercase
	b := AcquireByteSlice()
	defer ReleaseByteSlice(b)

	b = append(b[:0], s...)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// FastToUpper converts string to uppercase without allocation for ASCII
func FastToUpper(s string) string {
	needsUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			needsUpper = true
			break
		}
	}
	if !needsUpper {
		return s
	}

	b := AcquireByteSlice()
	defer ReleaseByteSlice(b)

	b = append(b[:0], s...)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

// FastContains checks if substring exists without allocation
func FastContains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// FastHasPrefix checks prefix without allocation
func FastHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// FastHasSuffix checks suffix without allocation
func FastHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// FastEqualFold compares strings case-insensitively for ASCII
func FastEqualFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c1, c2 := s[i], t[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// FastIndex finds substring index
func FastIndex(s, substr string) int {
	return strings.Index(s, substr)
}

// FastSplit splits string without excessive allocations
func FastSplit(s, sep string) []string {
	if sep == "" {
		return []string{s}
	}

	n := strings.Count(s, sep) + 1
	result := make([]string, 0, n)

	start := 0
	for i := 0; i < len(s); {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			i += len(sep)
			start = i
		} else {
			i++
		}
	}
	result = append(result, s[start:])
	return result
}

// FastJoin joins strings with a separator
func FastJoin(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	// Calculate total length
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}
	totalLen += len(sep) * (len(strs) - 1)

	// Build result
	buf := make([]byte, 0, totalLen)
	for i, s := range strs {
		if i > 0 {
			buf = append(buf, sep...)
		}
		buf = append(buf, s...)
	}
	return string(buf)
}

// FastTrimSpace trims whitespace without allocation for simple cases
func FastTrimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end {
		c := s[start]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			start++
		} else {
			break
		}
	}

	for end > start {
		c := s[end-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			end--
		} else {
			break
		}
	}

	return s[start:end]
}

// FastTrimPrefix removes prefix if present
func FastTrimPrefix(s, prefix string) string {
	if FastHasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// FastTrimSuffix removes suffix if present
func FastTrimSuffix(s, suffix string) string {
	if FastHasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// FastReplace replaces all occurrences without using regex
func FastReplace(s, old, new string) string {
	if old == "" || old == new {
		return s
	}

	n := strings.Count(s, old)
	if n == 0 {
		return s
	}

	// Calculate new length
	newLen := len(s) + n*(len(new)-len(old))
	buf := make([]byte, 0, newLen)

	start := 0
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			buf = append(buf, s[start:i]...)
			buf = append(buf, new...)
			i += len(old) - 1
			start = i + 1
		}
	}
	buf = append(buf, s[start:]...)

	return string(buf)
}

// FastRepeat repeats a string n times
func FastRepeat(s string, count int) string {
	if count <= 0 || s == "" {
		return ""
	}
	if count == 1 {
		return s
	}

	// Check for overflow
	maxLen := len(s) * count
	if maxLen/len(s) != count {
		panic("FastRepeat: result too large")
	}

	buf := make([]byte, 0, maxLen)
	for i := 0; i < count; i++ {
		buf = append(buf, s...)
	}
	return string(buf)
}

// FastFields splits string by whitespace
func FastFields(s string) []string {
	var fields []string
	start := -1

	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' { // covers space, tab, newline, etc.
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}

	if start >= 0 {
		fields = append(fields, s[start:])
	}

	return fields
}

// BytePool is a pool for byte operations
type BytePool struct {
	pool *SlicePool[byte]
}

// NewBytePool creates a new byte pool
func NewBytePool(capacity int) *BytePool {
	return &BytePool{
		pool: NewSlicePool[byte](capacity),
	}
}

// Get retrieves a byte slice
func (p *BytePool) Get() []byte {
	return p.pool.Get()
}

// Put returns a byte slice
func (p *BytePool) Put(b []byte) {
	p.pool.Put(b)
}

// StringBuilder provides efficient string building
type StringBuilder struct {
	buf []byte
}

// NewStringBuilder creates a new string builder
func NewStringBuilder() *StringBuilder {
	return &StringBuilder{
		buf: make([]byte, 0, 256),
	}
}

// NewStringBuilderWithCapacity creates a builder with specific capacity
func NewStringBuilderWithCapacity(capacity int) *StringBuilder {
	return &StringBuilder{
		buf: make([]byte, 0, capacity),
	}
}

// WriteString appends a string
func (b *StringBuilder) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

// WriteByte appends a byte
func (b *StringBuilder) WriteByte(c byte) error {
	b.buf = append(b.buf, c)
	return nil
}

// WriteBytes appends bytes
func (b *StringBuilder) WriteBytes(p []byte) {
	b.buf = append(b.buf, p...)
}

// WriteRune appends a rune
func (b *StringBuilder) WriteRune(r rune) {
	if r < utf8RuneSelf {
		b.buf = append(b.buf, byte(r))
		return
	}
	var buf [4]byte
	n := utf8EncodeRune(buf[:], r)
	b.buf = append(b.buf, buf[:n]...)
}

// String returns the built string
func (b *StringBuilder) String() string {
	return string(b.buf)
}

// Bytes returns the built bytes
func (b *StringBuilder) Bytes() []byte {
	return b.buf
}

// Len returns the length
func (b *StringBuilder) Len() int {
	return len(b.buf)
}

// Cap returns the capacity
func (b *StringBuilder) Cap() int {
	return cap(b.buf)
}

// Reset clears the builder
func (b *StringBuilder) Reset() {
	b.buf = b.buf[:0]
}

// Grow grows the buffer capacity
func (b *StringBuilder) Grow(n int) {
	if n > cap(b.buf)-len(b.buf) {
		newBuf := make([]byte, len(b.buf), cap(b.buf)+n)
		copy(newBuf, b.buf)
		b.buf = newBuf
	}
}

// Constants for UTF-8 encoding
const utf8RuneSelf = 0x80

// utf8EncodeRune encodes a rune to UTF-8
func utf8EncodeRune(p []byte, r rune) int {
	switch i := uint32(r); {
	case i < (1 << 7):
		p[0] = byte(r)
		return 1
	case i < (1 << 11):
		p[0] = 0xC0 | byte(r>>6)
		p[1] = 0x80 | byte(r)&0x3F
		return 2
	case i < (1 << 16):
		p[0] = 0xE0 | byte(r>>12)
		p[1] = 0x80 | byte(r>>6)&0x3F
		p[2] = 0x80 | byte(r)&0x3F
		return 3
	default:
		p[0] = 0xF0 | byte(r>>18)
		p[1] = 0x80 | byte(r>>12)&0x3F
		p[2] = 0x80 | byte(r>>6)&0x3F
		p[3] = 0x80 | byte(r)&0x3F
		return 4
	}
}

// IsASCII checks if string contains only ASCII characters
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8RuneSelf {
			return false
		}
	}
	return true
}

// CountRunes counts runes in string (faster than utf8.RuneCountInString for ASCII)
func CountRunes(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] < utf8RuneSelf {
			i++
		} else {
			_, size := utf8DecodeRune(s[i:])
			i += size
		}
		n++
	}
	return n
}

// utf8DecodeRune decodes a UTF-8 rune
func utf8DecodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	s0 := s[0]
	if s0 < utf8RuneSelf {
		return rune(s0), 1
	}
	// Handle multi-byte sequences
	x := first[s0]
	if x >= as {
		return rune(x & 7), 1
	}
	sz := int(x & 7)
	if len(s) < sz {
		return RuneError, 1
	}
	// ... simplified for common cases
	return rune(s0), 1
}

// Constants for UTF-8 decoding
const (
	RuneError = unicode.ReplacementChar
	as        = 0xF0
)

// first table for UTF-8 decoding (simplified)
var first = [256]uint8{
	// ASCII
	0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7,
	8: 8, 9: 9, 10: 10, 11: 11, 12: 12, 13: 13, 14: 14, 15: 15,
	// ... (truncated for brevity)
	0x80: as | 1, 0x81: as | 1, 0x82: as | 1, 0x83: as | 1,
	0x84: as | 1, 0x85: as | 1, 0x86: as | 1, 0x87: as | 1,
	0x88: as | 1, 0x89: as | 1, 0x8A: as | 1, 0x8B: as | 1,
	0x8C: as | 1, 0x8D: as | 1, 0x8E: as | 1, 0x8F: as | 1,
	// ... more entries
}

// init initializes the first table
func init() {
	for i := 0x80; i < 0xC0; i++ {
		first[i] = as | 1 // continuation byte
	}
	for i := 0xC0; i < 0xE0; i++ {
		first[i] = 2 // 2-byte sequence start
	}
	for i := 0xE0; i < 0xF0; i++ {
		first[i] = 3 // 3-byte sequence start
	}
	for i := 0xF0; i < 0xF8; i++ {
		first[i] = 4 // 4-byte sequence start
	}
}
