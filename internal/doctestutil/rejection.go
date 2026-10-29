//go:build test

package doctestutil

import (
	"errors"
	"fmt"
	"strings"
)

// RejectionDisplay is the category of what the extension shows for a reason
// the server rejects a request with, as a rejection-reason table writes it.
// The zero value is not a category: no cell maps to it.
type RejectionDisplay int

// The categories a rejection-reason table's display cell can name.
const (
	// DisplayFixedMessage is a message the extension keeps for the reason.
	DisplayFixedMessage RejectionDisplay = iota + 1
	// DisplayUnexpectedReply is the extension's message for a reply it does
	// not expect.
	DisplayUnexpectedReply
	// DisplayFixedMessageUnlessNoRequestID is a fixed message, or an
	// unexpected reply when the response's requestId is empty.
	DisplayFixedMessageUnlessNoRequestID
)

// String names the category in a failure message.
func (d RejectionDisplay) String() string {
	switch d {
	case DisplayFixedMessage:
		return "fixed message"
	case DisplayUnexpectedReply:
		return "unexpected reply"
	case DisplayFixedMessageUnlessNoRequestID:
		return "fixed message unless the requestId is empty"
	}
	return fmt.Sprintf("RejectionDisplay(%d)", int(d))
}

// ErrUnknownRejectionDisplay reports a display cell that is none of a
// language's spellings.
var ErrUnknownRejectionDisplay = errors.New("unknown rejection display")

// RejectionDisplaySpelling is how one language writes each category in a
// rejection-reason table.
type RejectionDisplaySpelling struct {
	FixedMessage                  string
	UnexpectedReply               string
	FixedMessageUnlessNoRequestID string
}

// The spellings of the two languages. development.md writes the Japanese one;
// the README's two-column table writes the language of its version.
var (
	RejectionDisplayJA = RejectionDisplaySpelling{
		FixedMessage:                  "固定のメッセージ",
		UnexpectedReply:               "想定外の応答",
		FixedMessageUnlessNoRequestID: "固定のメッセージ（`requestId` が空のときは想定外の応答）",
	}
	RejectionDisplayEN = RejectionDisplaySpelling{
		FixedMessage:                  "fixed message",
		UnexpectedReply:               "unexpected reply",
		FixedMessageUnlessNoRequestID: "fixed message (unexpected reply when `requestId` is empty)",
	}
)

// Category returns the category whose spelling equals cell, ignoring the
// space around it. A cell that is none of the spellings, the empty cell
// included, is an error wrapping ErrUnknownRejectionDisplay: it is not taken
// for the nearest category.
func (s RejectionDisplaySpelling) Category(cell string) (RejectionDisplay, error) {
	cell = strings.TrimSpace(cell)
	if cell != "" {
		switch cell {
		case s.FixedMessage:
			return DisplayFixedMessage, nil
		case s.UnexpectedReply:
			return DisplayUnexpectedReply, nil
		case s.FixedMessageUnlessNoRequestID:
			return DisplayFixedMessageUnlessNoRequestID, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrUnknownRejectionDisplay, cell)
}
