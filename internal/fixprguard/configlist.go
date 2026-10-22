package fixprguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// mainBranch is the branch whose settings are compared even when it is not
// the current branch.
const mainBranch = "main"

// branchSection is the section of a branch setting. git lists section names
// in lowercase, so a byte comparison is enough.
const branchSection = "branch"

// configListDigest parses the output of git config --list --show-origin -z,
// rejecting any other shape with errConfigList, and digests, in order, every
// entry except the branch settings of branches other than mainBranch and the
// branch whose name digest is branchDigest. A branchDigest that is not a
// digest is rejected rather than read as matching no branch.
//
// An entry is <origin> NUL <name> NUL for a setting without a value, or
// <origin> NUL <name> LF <value> NUL. Each compared entry enters the digest
// with its delimiters, so a setting without a value differs from an empty
// one and entry boundaries are unambiguous; a skipped entry adds nothing, so
// adding, changing or removing one leaves the digest as it was. The origin and
// the value are never interpreted.
func configListDigest(out []byte, branchDigest string) (string, error) {
	if !validHex(branchDigest, digestLength) {
		return "", errConfigList
	}
	if len(out) == 0 {
		return "", errConfigList
	}
	h := sha256.New()
	for rest := out; len(rest) > 0; {
		entry, name, next, err := nextConfigEntry(rest)
		if err != nil {
			return "", err
		}
		if name.compared(branchDigest) {
			_, _ = h.Write(entry)
		}
		rest = next
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// configName is a setting's name split at its first and last ".": the
// section is before the first, the subsection between the two (it may hold
// "." or be empty), and the variable after the last. hasSubsection is false
// for a name with a single ".".
type configName struct {
	section, subsection []byte
	hasSubsection       bool
}

// nextConfigEntry splits the first entry off out and returns it whole, its
// name, and the rest. A missing delimiter, an empty origin, or a name that is
// not <section>.<variable> or <section>.<subsection>.<variable> with a
// non-empty section and variable is rejected.
func nextConfigEntry(out []byte) (entry []byte, name configName, rest []byte, err error) {
	// Without a NUL after the origin nothing follows it, which the name-end
	// check below rejects.
	origin, afterOrigin, _ := bytes.Cut(out, []byte{0})
	if len(origin) == 0 {
		return nil, configName{}, nil, errConfigList
	}
	end := indexNameEnd(afterOrigin)
	if end < 0 {
		return nil, configName{}, nil, errConfigList
	}
	name, ok := splitConfigName(afterOrigin[:end])
	if !ok {
		return nil, configName{}, nil, errConfigList
	}
	rest = afterOrigin[end+1:]
	if afterOrigin[end] == '\n' {
		_, after, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return nil, configName{}, nil, errConfigList
		}
		rest = after
	}
	return out[:len(out)-len(rest)], name, rest, nil
}

// indexNameEnd returns the index of the LF or NUL that ends a name, or -1.
func indexNameEnd(b []byte) int {
	for i, c := range b {
		if c == '\n' || c == 0 {
			return i
		}
	}
	return -1
}

// splitConfigName splits a name into a configName. ok is false for a name
// without "." or with an empty section or variable.
func splitConfigName(name []byte) (configName, bool) {
	first := bytes.IndexByte(name, '.')
	last := bytes.LastIndexByte(name, '.')
	switch {
	case first < 0: // no "." at all, an empty name included
		return configName{}, false
	case first == 0: // empty section
		return configName{}, false
	case last == len(name)-1: // empty variable
		return configName{}, false
	case first == last:
		return configName{section: name[:first]}, true
	}
	return configName{section: name[:first], subsection: name[first+1 : last], hasSubsection: true}, true
}

// compared reports whether the setting is compared: every setting except a
// branch setting whose subsection, as bytes, is neither mainBranch nor the
// branch whose name digest is branchDigest.
func (n configName) compared(branchDigest string) bool {
	if string(n.section) != branchSection || !n.hasSubsection {
		return true
	}
	return string(n.subsection) == mainBranch || digestString(string(n.subsection)) == branchDigest
}
