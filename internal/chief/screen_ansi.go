package chief

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Errors of the ANSI screen reader. A screen that holds anything other than
// SGR sequences, CR LF line ends, and valid UTF-8 is not a shape the samples
// showed, so it is rejected rather than cleaned up.
var (
	errScreenEncoding = errors.New("chief: screen is not valid UTF-8")
	errScreenEscape   = errors.New("chief: screen holds an escape sequence other than SGR")
	errScreenSGR      = errors.New("chief: screen holds an SGR parameter outside the accepted set")
	errScreenTooLarge = errors.New("chief: screen exceeds the read cap")
)

// rgb is a 24-bit foreground color. set is false for the terminal default and
// for indexed colors, which the readers never compare.
type rgb struct {
	set     bool
	r, g, b uint8
}

// cell is one character of a screen line with the attributes the readers use.
type cell struct {
	r   rune
	dim bool
	fg  rgb
}

// screenLine is one line of a screen: its cells and their text with the
// trailing blanks a terminal pads lines with removed.
type screenLine struct {
	cells []cell
	text  string
}

// sgrState is the part of the SGR state the readers use.
type sgrState struct {
	dim bool
	fg  rgb
}

// parseANSIScreen splits a screen read with "herdr agent read --format ansi"
// into lines. Contract: it accepts only SGR sequences (ESC [ params m), lines
// ended by LF with an optional CR before it, valid UTF-8, and no character
// Sanitize would replace other than tab; and it refuses a screen of more than
// screenMaxLines lines, or whose text (SGR sequences removed, trailing blanks
// trimmed, one byte per line end) exceeds screenMaxBytes bytes, since a cut
// screen is never read as a shape. The raw read is capped by the Herdr reader
// that calls it.
func parseANSIScreen(raw string) ([]screenLine, error) {
	if !utf8.ValidString(raw) {
		return nil, errScreenEncoding
	}
	body := strings.TrimSuffix(raw, "\n")
	parts := strings.Split(body, "\n")
	if len(parts) > screenMaxLines {
		return nil, errScreenTooLarge
	}
	var state sgrState
	lines := make([]screenLine, 0, len(parts))
	textBytes := 0
	for _, part := range parts {
		part = strings.TrimSuffix(part, "\r")
		line, err := parseANSILine(part, &state)
		if err != nil {
			return nil, err
		}
		textBytes += len(line.text) + 1
		if textBytes > screenMaxBytes {
			return nil, errScreenTooLarge
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// parseANSILine reads one line, carrying the SGR state across lines as a
// terminal does.
func parseANSILine(part string, state *sgrState) (screenLine, error) {
	var cells []cell
	for i := 0; i < len(part); {
		if part[i] == csiIntroducer[0] {
			n, err := applySGR(part[i:], state)
			if err != nil {
				return screenLine{}, err
			}
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(part[i:])
		if hiddenRune(r) && r != '\t' {
			return screenLine{}, errScreenEscape
		}
		cells = append(cells, cell{r: r, dim: state.dim, fg: state.fg})
		i += size
	}
	end := len(cells)
	for end > 0 && cells[end-1].r == ' ' {
		end--
	}
	cells = cells[:end]
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.r)
	}
	return screenLine{cells: cells, text: b.String()}, nil
}

// csiIntroducer starts every control sequence the screens hold.
const csiIntroducer = "\x1b["

// applySGR reads one "ESC [ params m" sequence at the start of s, applies it
// to state, and returns its length in bytes.
func applySGR(s string, state *sgrState) (int, error) {
	rest, ok := strings.CutPrefix(s, csiIntroducer)
	if !ok {
		return 0, errScreenEscape
	}
	end := strings.IndexFunc(rest, func(r rune) bool {
		return (r < '0' || r > '9') && r != ';'
	})
	if end < 0 || rest[end] != 'm' {
		return 0, errScreenEscape
	}
	if err := applySGRParams(rest[:end], state); err != nil {
		return 0, err
	}
	return len(csiIntroducer) + end + len("m"), nil
}

// SGR parameters the samples showed. The other accepted parameters change
// only what the readers never compare and are listed in sgrIgnored.
const (
	sgrReset         = 0
	sgrDim           = 2
	sgrNormal        = 22
	sgrFgExtended    = 38
	sgrBgExtended    = 48
	colorModeIndexed = "5"
	colorModeRGB     = "2"
	rgbComponents    = 3
	sgrParamsIndexed = 2 // the mode and the index
	sgrParamsRGB     = 1 + rgbComponents
)

// sgrFgDefaultCodes are the foreground parameters that set a non-24-bit
// color: 30-37, 39, and 90-97.
var sgrFgDefaultCodes = codeSet([]int{30, 31, 32, 33, 34, 35, 36, 37, 39, 90, 91, 92, 93, 94, 95, 96, 97})

// sgrIgnored are the parameters that change only what the readers never
// compare: bold, italic, underline, reverse, strike and their resets, and the
// background colors other than the extended forms.
var sgrIgnored = codeSet([]int{
	1, 3, 4, 7, 9, 23, 24, 27, 29, 49,
	40, 41, 42, 43, 44, 45, 46, 47, 100, 101, 102, 103, 104, 105, 106, 107,
})

// codeSet makes a set of SGR codes.
func codeSet(codes []int) map[int]struct{} {
	set := make(map[int]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return set
}

// applySGRParams applies the parameters of one SGR sequence. It accepts the
// parameters the samples showed and the other attribute and color codes of
// the same kinds, which change nothing the readers compare except dim (2 and
// its reset 22) and the 24-bit foreground.
func applySGRParams(params string, state *sgrState) error {
	if params == "" {
		*state = sgrState{}
		return nil
	}
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		code, err := strconv.Atoi(fields[i])
		if err != nil {
			return errScreenSGR
		}
		if _, ok := sgrIgnored[code]; ok {
			continue
		}
		if _, ok := sgrFgDefaultCodes[code]; ok {
			state.fg = rgb{}
			continue
		}
		switch code {
		case sgrReset:
			*state = sgrState{}
		case sgrDim:
			state.dim = true
		case sgrNormal:
			state.dim = false
		case sgrFgExtended, sgrBgExtended:
			consumed, color, err := extendedColor(fields[i+1:])
			if err != nil {
				return err
			}
			if code == sgrFgExtended {
				state.fg = color
			}
			i += consumed
		default:
			return errScreenSGR
		}
	}
	return nil
}

// extendedColor reads the parameters after 38 or 48: "5;n" (indexed) or
// "2;r;g;b" (24-bit). It returns how many fields it read.
func extendedColor(fields []string) (int, rgb, error) {
	switch {
	case len(fields) >= sgrParamsIndexed && fields[0] == colorModeIndexed:
		if _, err := byteParam(fields[1]); err != nil {
			return 0, rgb{}, err
		}
		return sgrParamsIndexed, rgb{}, nil
	case len(fields) >= sgrParamsRGB && fields[0] == colorModeRGB:
		var c [rgbComponents]uint8
		for k := range rgbComponents {
			v, err := byteParam(fields[1+k])
			if err != nil {
				return 0, rgb{}, err
			}
			c[k] = v
		}
		return sgrParamsRGB, rgb{set: true, r: c[0], g: c[1], b: c[2]}, nil
	default:
		return 0, rgb{}, errScreenSGR
	}
}

// byteParam reads one color component from 0 to 255.
func byteParam(s string) (uint8, error) {
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, errScreenSGR
	}
	return uint8(v), nil
}

// screenSummaryLines is how many lines of a screen a stop or a result keeps.
const screenSummaryLines = 40

// screenSummary returns the last screenSummaryLines lines of a screen's text,
// its colors and attributes removed, for showing what a pane showed when a
// send failed. A screen the reader cannot split gives "".
func screenSummary(raw string) string {
	lines, err := parseANSIScreen(raw)
	if err != nil {
		return ""
	}
	lines = trimmedLines(lines)
	texts := make([]string, 0, screenSummaryLines)
	for _, l := range lines[max(0, len(lines)-screenSummaryLines):] {
		texts = append(texts, l.text)
	}
	return joinTexts(texts)
}
