// Package wire loads a capture of JSON-RPC messages as seen on the MCP
// boundary. Spans never carry the raw params._meta values, so checks on the
// wire format of trace context (SEP-414) need this second input.
//
// Capture format: newline-delimited JSON, one object per received message:
//
//	{"method":"tools/call","_meta":{"traceparent":"00-...-01"}}
//
// Any extra fields are ignored. The harness in examples/ writes this with
// the -capture flag; other producers (proxies, SDK hooks) can emit the same.
package wire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Message is one captured JSON-RPC request or notification.
type Message struct {
	Method string         `json:"method"`
	Meta   map[string]any `json:"_meta"`
	Line   int            `json:"-"`
}

// LoadFile reads a capture file.
func LoadFile(path string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load reads newline-delimited capture records. Blank lines are skipped.
func Load(r io.Reader) ([]Message, error) {
	var out []Message
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 16<<20)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var m Message
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		m.Line = line
		out = append(out, m)
	}
	return out, sc.Err()
}
