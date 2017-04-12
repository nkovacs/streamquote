// Package streamquote implements a streaming version of strconv.Quote.
package streamquote

import (
	"io"
	"strconv"
	"unicode/utf8"
)

// Converter converts data by escaping control characters and
// non-printable characters using Go escape sequences.
type Converter interface {
	// Convert converts the data in "in", writing it to "out".
	// It uses Go escape sequences (\t, \n, \xFF, \u0100) for control characters
	// and non-printable characters as defined by strconv.IsPrint.
	// It is not safe for concurrent use.
	Convert(in io.Reader, out io.Writer) (int, error)
}

type converter struct {
	buffer [bufSize]byte
}

const bufSize = 100 * 1024

const lowerhex = "0123456789abcdef"

// New returns a new Converter.
func New() Converter {
	return &converter{}
}

// Convert converts the data in "in", writing it to "out".
// It uses Go escape sequences (\t, \n, \xFF, \u0100) for control characters
// and non-printable characters as defined by strconv.IsPrint.
// It is not safe for concurrent use.
func (c *converter) Convert(in io.Reader, out io.Writer) (int, error) {
	var err error
	bufSize := len(c.buffer)
	n := 0

	var processed = bufSize
	var dataLen = 0

	for {
		if processed+utf8.UTFMax > bufSize {
			// need to read more
			leftover := bufSize - processed
			if leftover > 0 {
				copy(c.buffer[:leftover], c.buffer[processed:])
			}
			read, peekErr := in.Read(c.buffer[leftover:])
			if peekErr != nil && peekErr != io.EOF {
				err = peekErr
				break
			}
			dataLen = leftover + read
			processed = 0
		}
		if dataLen-processed == 0 {
			break
		}

		maxRune := processed + utf8.UTFMax
		if maxRune > dataLen {
			maxRune = dataLen
		}
		data := c.buffer[processed:maxRune]

		var discard, n2 int
		r, width := utf8.DecodeRune(data)
		if width == 1 && r == utf8.RuneError {
			out.Write([]byte{'\\', 'x', lowerhex[data[0]>>4], lowerhex[data[0]&0xF]})
			n2 = 4
			discard = 1
		} else {
			discard = width
			if r == rune('"') || r == '\\' { // always backslashed
				out.Write([]byte{'\\', byte(r)})
				n2 = 2
			} else if strconv.IsPrint(r) {
				out.Write(data[:width])
				n2 = width
			} else {
				switch r {
				case '\a':
					out.Write([]byte{'\\', 'a'})
					n2 = 2
				case '\b':
					out.Write([]byte{'\\', 'b'})
					n2 = 2
				case '\f':
					out.Write([]byte{'\\', 'f'})
					n2 = 2
				case '\n':
					out.Write([]byte{'\\', 'n'})
					n2 = 2
				case '\r':
					out.Write([]byte{'\\', 'r'})
					n2 = 2
				case '\t':
					out.Write([]byte{'\\', 't'})
					n2 = 2
				case '\v':
					out.Write([]byte{'\\', 'v'})
					n2 = 2
				default:
					switch {
					case r < ' ':
						out.Write([]byte{'\\', 'x', lowerhex[data[0]>>4], lowerhex[data[0]&0xF]})
						n2 = 4
					case r > utf8.MaxRune:
						r = 0xFFFD
						fallthrough
					case r < 0x10000:
						out.Write([]byte{'\\', 'u'})
						n2 = 2
						for s := 12; s >= 0; s -= 4 {
							out.Write([]byte{lowerhex[r>>uint(s)&0xF]})
							n2++
						}
					default:
						out.Write([]byte{'\\', 'U'})
						n2 = 2
						for s := 28; s >= 0; s -= 4 {
							out.Write([]byte{lowerhex[r>>uint(s)&0xF]})
							n2++
						}
					}
				}
			}
		}
		processed += discard
		n += n2
	}

	return n, err
}
