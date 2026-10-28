//go:build test

package nmhost

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// skipIfRoot skips a test that depends on permissions, which root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not hold for root")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the temporary directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, original) })
}

// HostWireValuesForTest holds the wire values of the host's replies, read from
// the replies the host marshals, for the guard that compares them with the
// extension's declarations.
type HostWireValuesForTest struct {
	Version    int      // the "v" member
	Kinds      []string // the "kind" member of each shape: response, then error
	HostErrors []string // the "error" member of each HostError constant
}

// WireValuesForTest marshals one reply of each shape and one error reply per
// HostError constant, and returns the members the extension must agree with.
// The constants are found by counting up from the zero value until Value falls
// back to protocol_error, so a constant added after the last one is included.
func WireValuesForTest(t *testing.T) HostWireValuesForTest {
	t.Helper()
	type members struct {
		V     int    `json:"v"`
		Kind  string `json:"kind"`
		Error string `json:"error"`
	}
	decode := func(reply any) members {
		var m members
		if err := json.Unmarshal(encodeReply(reply), &m); err != nil {
			t.Fatalf("decode the host reply: %v", err)
		}
		return m
	}
	response := decode(newResponseReply(http.StatusAccepted, nil))
	values := HostWireValuesForTest{Version: response.V, Kinds: []string{response.Kind}}
	for e := HostErrorProtocol; e == HostErrorProtocol || e.Value() != HostErrorProtocol.Value(); e++ {
		reply := decode(newErrorReply(e))
		if reply.V != values.Version {
			t.Fatalf("the %s error reply has version %d, the response reply %d", reply.Error, reply.V, values.Version)
		}
		if e == HostErrorProtocol {
			values.Kinds = append(values.Kinds, reply.Kind)
		}
		values.HostErrors = append(values.HostErrors, reply.Error)
	}
	return values
}

// MaxResponseBytesForTest is the largest server response body the host
// relays, for the guard in cmd/yt2column-server that compares it with the
// server's status response limit.
const MaxResponseBytesForTest = maxResponseBytes

// LargestReplyForTest marshals the response reply that carries a server
// response body of MaxResponseBytesForTest bytes with the largest status net/http names (511),
// and returns its size and the browser's message limit it must fit. It
// marshals the reply directly, because encodeReply would replace one over the
// limit with the protocol_error reply.
func LargestReplyForTest(t *testing.T) (replyBytes, frameLimit int) {
	t.Helper()
	payload, err := json.Marshal(newResponseReply(http.StatusNetworkAuthenticationRequired, bytes.Repeat([]byte{0xff}, maxResponseBytes)))
	if err != nil {
		t.Fatalf("marshal the largest reply: %v", err)
	}
	return len(payload), maxFrameBytes
}
