// escape_untrusted escapes untrusted bytes read from standard input for safe
// terminal and log display.
//
// Contract: every byte sequence representing Unicode general category Cc
// (including tab and line endings), Cf, Zl, or Zp, a Unicode noncharacter, or
// an invalid UTF-8 byte is emitted byte-by-byte as \xHH (lowercase hex), and
// literal backslashes are doubled to \\. Valid printable runes and valid
// U+FFFD pass through unchanged.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// isNoncharacter reports whether r is a Unicode noncharacter (U+FDD0..U+FDEF
// or the last two code points of any plane).
func isNoncharacter(r rune) bool {
	return (r >= 0xFDD0 && r <= 0xFDEF) || r&0xFFFE == 0xFFFE
}

// mustEscape reports whether rune r must be byte-escaped. Unlike
// internal/mergepr's invisible check, tab (U+0009) is in Cc and is escaped.
func mustEscape(r rune) bool {
	return isNoncharacter(r) || unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
}

func writeEscapedBytes(w *bufio.Writer, b []byte) error {
	for _, c := range b {
		if _, err := fmt.Fprintf(w, `\x%02x`, c); err != nil {
			return err
		}
	}
	return nil
}

func escapeStream(r io.Reader, w *bufio.Writer, trimFinalLF bool) error {
	buf := make([]byte, 32*1024)
	n := 0
	pendingLF := false

	flushPendingLF := func() error {
		if !pendingLF {
			return nil
		}
		pendingLF = false
		_, err := w.WriteString(`\x0a`)
		return err
	}

	for {
		nr, readErr := r.Read(buf[n:])
		n += nr
		eof := readErr == io.EOF
		if readErr != nil && !eof {
			return readErr
		}

		i := 0
		for i < n {
			if !eof && !utf8.FullRune(buf[i:n]) {
				break
			}
			rn, size := utf8.DecodeRune(buf[i:n])
			if trimFinalLF && rn == '\n' && size == 1 {
				if err := flushPendingLF(); err != nil {
					return err
				}
				pendingLF = true
				i += size
				continue
			}
			if err := flushPendingLF(); err != nil {
				return err
			}
			switch {
			case rn == utf8.RuneError && size == 1:
				if err := writeEscapedBytes(w, buf[i:i+1]); err != nil {
					return err
				}
			case mustEscape(rn):
				if err := writeEscapedBytes(w, buf[i:i+size]); err != nil {
					return err
				}
			case rn == '\\':
				if _, err := w.WriteString(`\\`); err != nil {
					return err
				}
			default:
				if _, err := w.Write(buf[i : i+size]); err != nil {
					return err
				}
			}
			i += size
		}

		copy(buf, buf[i:n])
		n -= i

		if eof {
			break
		}
	}

	return w.Flush()
}

func main() {
	trimFinalLF := flag.Bool("trim-final-lf", false, "drop a single trailing LF byte before escaping")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "escape_untrusted: unexpected positional arguments")
		os.Exit(2)
	}

	w := bufio.NewWriter(os.Stdout)
	if err := escapeStream(os.Stdin, w, *trimFinalLF); err != nil {
		fmt.Fprintf(os.Stderr, "escape_untrusted: %v\n", err)
		os.Exit(1)
	}
}
