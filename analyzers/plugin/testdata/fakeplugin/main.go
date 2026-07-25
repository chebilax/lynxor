// fakeplugin is a test-only stand-in for a real plugin subprocess, used by
// plugin_test.go to exercise Plugin's protocol handling (handshake, file
// requests, configure, and every fault mode) without shelling out to Python
// or hand-rolling protocol violations in a shell script. Behavior is
// selected via the LYNXOR_FAKE_PLUGIN_SCENARIO environment variable, which
// Load's exec.Command inherits from the test process.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type msg map[string]any

func writeLine(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func main() {
	scenario := os.Getenv("LYNXOR_FAKE_PLUGIN_SCENARIO")
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 8<<20)

	if !in.Scan() {
		return
	}

	switch scenario {
	case "bad_handshake_type":
		writeLine(msg{"type": "unexpected"})
		return
	case "bad_handshake_error":
		writeLine(msg{"type": "error", "message": "nope, not doing that"})
		return
	case "bad_handshake_no_name":
		writeLine(msg{"type": "hello_ack"})
		return
	case "crash_before_ack":
		os.Exit(1)
	default:
		writeLine(msg{"type": "hello_ack", "plugin_name": "fakeplugin", "plugin_version": "0.0.1"})
	}

	for in.Scan() {
		var m msg
		if err := json.Unmarshal(in.Bytes(), &m); err != nil {
			continue
		}
		if mtype, _ := m["type"].(string); mtype == "configure" {
			continue // fire-and-forget, no response
		}

		path, _ := m["path"].(string)
		switch scenario {
		case "timeout":
			time.Sleep(10 * time.Second) // longer than plugin's requestTimeout
			return
		case "crash_on_file":
			os.Exit(1)
		case "malformed_json":
			fmt.Println("{not valid json")
		case "unexpected_type":
			writeLine(msg{"type": "weird"})
		case "wrong_path":
			writeLine(msg{"type": "result", "path": path + "-wrong", "findings": []any{}})
		case "fatal_error":
			writeLine(msg{"type": "error", "fatal": true, "message": "boom"})
		case "nonfatal_error":
			writeLine(msg{"type": "error", "fatal": false, "message": "meh, skipping this one"})
		case "wrong_severity":
			writeLine(msg{"type": "result", "path": path, "findings": []msg{
				{"id": "x", "severity": "WOWZA", "title": "t", "message": "m", "fix": "f"},
			}})
		case "empty_message":
			writeLine(msg{"type": "result", "path": path, "findings": []msg{
				{"id": "x", "severity": "HIGH", "title": "t", "message": "", "fix": "f"},
			}})
		case "no_prefix_no_category":
			writeLine(msg{"type": "result", "path": path, "findings": []msg{
				{"id": "myid", "severity": "CRITICAL", "title": "t", "message": "m", "fix": "f"},
			}})
		default: // "ok"
			writeLine(msg{"type": "result", "path": path, "findings": []msg{
				{"id": "fakeplugin.rule", "severity": "HIGH", "title": "t", "message": "m", "fix": "f", "line": 1, "category": "custom"},
			}})
		}
	}
}
