package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// approval is one line of the approval record: when the human approved, the
// configs approved, and the limit of the spend.
type approval struct {
	At      time.Time
	Configs []string
	Limit   micro
}

// errApproval reports an approval record that is missing, empty, or does not
// match its form; launch stops on it.
var errApproval = errors.New("approval record is missing or does not match its form")

// parseApprovals reads the approval record: one approval per line,
// "<RFC 3339 time>\t<config>,<config>,...\t<limit in USD>". The record is
// only appended to (plan section 1.2 item 8): the first line is the first approval,
// and the last line holds the configs and the limit in effect.
func parseApprovals(r io.Reader) ([]approval, error) {
	var out []approval
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 { //nolint:mnd // three columns
			return nil, fmt.Errorf("%w: line %q", errApproval, scanner.Text())
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			return nil, fmt.Errorf("%w: time: %w", errApproval, err)
		}
		configs := strings.Split(fields[1], ",")
		if slices.Contains(configs, "") {
			return nil, fmt.Errorf("%w: empty config in %q", errApproval, fields[1])
		}
		limit, err := parseMicro(fields[2])
		if err != nil {
			return nil, fmt.Errorf("%w: limit: %w", errApproval, err)
		}
		if len(out) > 0 && at.Before(out[len(out)-1].At) {
			return nil, fmt.Errorf("%w: approvals out of time order", errApproval)
		}
		out = append(out, approval{At: at, Configs: configs, Limit: limit})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no approval", errApproval)
	}
	return out, nil
}
