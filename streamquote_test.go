package streamquote

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// Taken from stdlib's strconv/quote_test.go
type quoteTest struct {
	in      string
	out     string
	ascii   string
	graphic string
}

var quotetests = []quoteTest{
	{"\a\b\f\r\n\t\v", `"\a\b\f\r\n\t\v"`, `"\a\b\f\r\n\t\v"`, `"\a\b\f\r\n\t\v"`},
	{"\\", `"\\"`, `"\\"`, `"\\"`},
	{"abc\xffdef", `"abc\xffdef"`, `"abc\xffdef"`, `"abc\xffdef"`},
	{"\u263a", `"☺"`, `"\u263a"`, `"☺"`},
	{"\U0010ffff", `"\U0010ffff"`, `"\U0010ffff"`, `"\U0010ffff"`},
	{"\x04", `"\x04"`, `"\x04"`, `"\x04"`},
	{"\x7f", `"\x7f"`, `"\x7f"`, `"\x7f"`},
	// Some non-printable but graphic runes. Final column is double-quoted.
	{"!\u00a0!\u2000!\u3000!", `"!\u00a0!\u2000!\u3000!"`, `"!\u00a0!\u2000!\u3000!"`, "\"!\u00a0!\u2000!\u3000!\""},
}

func TestConverter(t *testing.T) {
	converter := New()

	for _, tt := range quotetests {
		var buffer bytes.Buffer
		converter.Convert(strings.NewReader(tt.in), &buffer)
		expected := tt.out[1 : len(tt.out)-1]
		if out := buffer.String(); !testEqual(out, expected) {
			t.Errorf("Quote(%s) = %s, want %s", tt.in, out, expected)
		}
	}
}

// Size of the large string for benchmarking.
const largeSize = 10 * 1024 * 1024

// random seed for large string, to make the benchmark consistent.
const randSeed = 47

type randomRuneProvider struct {
	r     *rand.Rand
	limit int
	read  int
}

func (r *randomRuneProvider) Read(p []byte) (n int, err error) {
	n = len(p)
	available := r.limit - r.read
	if available < n {
		n = available
		err = io.EOF
	}
	for i := 0; i < n; i++ {
		p[i] = byte(r.r.Int63())
	}
	r.read += n
	return
}

func generateLargeString() io.Reader {
	source := rand.NewSource(randSeed)
	r := rand.New(source)

	return &randomRuneProvider{
		r:     r,
		limit: largeSize,
	}
}

// TestLargeString tests that converter and strconv.Quote
// return the same result for the large string.
func TestLargeString(t *testing.T) {
	largeStringReader := generateLargeString()
	b, err := ioutil.ReadAll(largeStringReader)
	if err != nil {
		t.Fatalf("Failed to read large string into buffer: %v", err)
	}
	expected := strconv.Quote(string(b))

	buffer := bytes.NewBufferString("\"")
	converter := New()

	largeStringReader = generateLargeString()
	_, err = converter.Convert(largeStringReader, buffer)
	if err != nil {
		t.Fatalf("Converter failed: %v", err)
	}
	buffer.WriteRune('"')
	got := buffer.String()
	if !testEqual(got, expected) {
		t.Fatalf("Large string does not match")
	}
}

func BenchmarkConverterSmall(b *testing.B) {
	converter := New()
	r := strings.NewReader("\a\b\f\r\n\t\v\a\b\f\r\n\t\v\a\b\f\r\n\t\v")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		converter.Convert(r, ioutil.Discard)
		r.Seek(0, 0)
	}
}

func BenchmarkStrconvQuoteSmall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strconv.Quote("\a\b\f\r\n\t\v\a\b\f\r\n\t\v\a\b\f\r\n\t\v")
	}
}

func BenchmarkConverterLarge(b *testing.B) {
	converter := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Using ioutil.Discard here may not seem fair,
		// because strconv.Quote has to allocate the result string,
		// but the point of this package is that you don't have to
		// allocate the entire result string, you can stream the result.
		converter.Convert(generateLargeString(), ioutil.Discard)
	}
}

func BenchmarkStrconvQuoteLarge(b *testing.B) {
	largeStringReader := generateLargeString()
	bs, err := ioutil.ReadAll(largeStringReader)
	if err != nil {
		b.Fatalf("Failed to read large string into buffer: %v", err)
	}
	s := string(bs)

	for i := 0; i < b.N; i++ {
		strconv.Quote(s)
	}
}

var testEqual = func(a string, b string) bool {
	return a == b
}

// testEqualOldGo compares strings byte-wise like ==, but considers `\x7f` and `\u007f` equal
func testEqualOldGo(a string, b string) bool {
	lenA := len(a)
	lenB := len(b)
	aI, bI := 0, 0
	for aI < lenA && bI < lenB {
		if aI+4 <= lenA && bI+6 <= lenB && a[aI:aI+4] == `\x7f` && b[bI:bI+6] == `\u007f` {
			aI += 4
			bI += 6
		} else if aI+6 <= lenA && bI+4 <= lenB && a[aI:aI+6] == `\u007f` && b[bI:bI+4] == `\x7f` {
			aI += 6
			bI += 4
		} else if a[aI] == b[bI] {
			aI++
			bI++
		} else {
			return false
		}
	}
	return aI == lenA && bI == lenB
}

func init() {
	if strconv.Quote("\x7f") == `"\u007f"` {
		testEqual = testEqualOldGo
	}
}

func TestTestEqualOldGo(t *testing.T) {
	test := func(a string, b string, eq bool) {
		actual := testEqualOldGo(a, b)
		if eq != actual {
			t.Fatalf("%v == %v, expected %v, got %v", a, b, eq, actual)
		}
		actual = testEqualOldGo(b, a)
		if eq != actual {
			t.Fatalf("%v == %v, expected %v, got %v", b, a, eq, actual)
		}
	}

	test("\\x7f", "\\u007f", true)
	test("\\x7f", "\\u007fa", false)
	test("\\x7fa", "\\u007f", false)
	test("a\\x7f", "\\u007f", false)
	test("\\x7f", "a\\u007f", false)
	test("\\x7fa", "\\u007fb", false)
	test("a\\x7f", "b\\u007f", false)
	test("\\x7f\\u007f", "\\u007f\\x7f", true)
	test("\\x7", "\\u007f", false)
}
