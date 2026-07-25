package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chebilax/lynxor/core"
)

// fakePluginPath is built once in TestMain and reused by every test that
// needs a real subprocess: Load shells out via exec.Command, so there's no
// way to exercise the handshake/protocol handling without a real
// executable on the other end of the pipe. See testdata/fakeplugin.
var fakePluginPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakeplugin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	fakePluginPath = filepath.Join(dir, "fakeplugin")
	build := exec.Command("go", "build", "-o", fakePluginPath, "./testdata/fakeplugin")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building fakeplugin fixture:", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func load(t *testing.T, scenario string) (*Plugin, error) {
	t.Helper()
	t.Setenv("LYNXOR_FAKE_PLUGIN_SCENARIO", scenario)
	return Load(fakePluginPath)
}

func TestLoad_Success(t *testing.T) {
	p, err := load(t, "ok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	if p.Name() != "fakeplugin" {
		t.Errorf("Name() = %q, want %q", p.Name(), "fakeplugin")
	}
}

func TestLoad_HandshakeFailures(t *testing.T) {
	cases := []string{
		"bad_handshake_type",
		"bad_handshake_error",
		"bad_handshake_no_name",
		"crash_before_ack",
	}
	for _, scenario := range cases {
		t.Run(scenario, func(t *testing.T) {
			p, err := load(t, scenario)
			if err == nil {
				p.Close()
				t.Fatalf("Load(%s): got nil error, want a handshake failure", scenario)
			}
		})
	}
}

func TestLoad_NonexistentExecutable(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("Load: got nil error for a nonexistent executable")
	}
}

func TestPlugin_Run_Success(t *testing.T) {
	p, err := load(t, "ok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	findings := p.Run(core.FileContext{Path: "main.go", Content: []byte("package main")})
	if len(findings) != 1 {
		t.Fatalf("Run: got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.ID != "fakeplugin.rule" {
		t.Errorf("ID = %q, want %q (already prefixed, left alone)", f.ID, "fakeplugin.rule")
	}
	if f.Severity != core.High {
		t.Errorf("Severity = %q, want %q", f.Severity, core.High)
	}
	if f.File != "main.go" {
		t.Errorf("File = %q, want %q", f.File, "main.go")
	}
	if f.Category != "custom" {
		t.Errorf("Category = %q, want %q (explicit category respected)", f.Category, "custom")
	}
	if !p.alive {
		t.Error("plugin should still be alive after a well-formed result")
	}
}

func TestPlugin_Run_IDGetsPrefixedAndCategoryDefaultsToPluginName(t *testing.T) {
	p, err := load(t, "no_prefix_no_category")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	findings := p.Run(core.FileContext{Path: "f.go"})
	if len(findings) != 1 {
		t.Fatalf("Run: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.ID != "fakeplugin.myid" {
		t.Errorf("ID = %q, want %q (prefixed since the plugin didn't)", f.ID, "fakeplugin.myid")
	}
	if f.Category != "fakeplugin" {
		t.Errorf("Category = %q, want %q (defaults to plugin name when omitted)", f.Category, "fakeplugin")
	}
}

func TestPlugin_Run_AbandonsOnProtocolFaults(t *testing.T) {
	cases := []string{
		"malformed_json",
		"unexpected_type",
		"wrong_path",
		"fatal_error",
		"crash_on_file",
	}
	for _, scenario := range cases {
		t.Run(scenario, func(t *testing.T) {
			p, err := load(t, scenario)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			defer p.Close()

			findings := p.Run(core.FileContext{Path: "f.go"})
			if findings != nil {
				t.Errorf("Run(%s): got %v findings, want nil", scenario, findings)
			}
			if p.alive {
				t.Errorf("Run(%s): plugin should be abandoned (alive=false)", scenario)
			}

			// A second Run on an abandoned plugin must be a no-op, not a
			// second attempt to talk to the (now-dead) process.
			if findings := p.Run(core.FileContext{Path: "g.go"}); findings != nil {
				t.Errorf("Run after abandon: got %v, want nil", findings)
			}
		})
	}
}

func TestPlugin_Run_NonFatalErrorDoesNotAbandon(t *testing.T) {
	p, err := load(t, "nonfatal_error")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	findings := p.Run(core.FileContext{Path: "f.go"})
	if findings != nil {
		t.Errorf("Run: got %v findings, want nil", findings)
	}
	if !p.alive {
		t.Error("a non-fatal error must not abandon the plugin")
	}
}

func TestPlugin_Run_DroppedFindings(t *testing.T) {
	cases := []string{"wrong_severity", "empty_message"}
	for _, scenario := range cases {
		t.Run(scenario, func(t *testing.T) {
			p, err := load(t, scenario)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			defer p.Close()

			findings := p.Run(core.FileContext{Path: "f.go"})
			if findings != nil {
				t.Errorf("Run(%s): got %v findings, want nil (dropped, not a protocol fault)", scenario, findings)
			}
			if !p.alive {
				t.Errorf("Run(%s): a dropped finding is not a protocol fault, plugin should stay alive", scenario)
			}
		})
	}
}

// Deliberately slow (bounded by the real requestTimeout, not a shortened
// test-only value -- there's no way to configure it without changing
// production code): confirms Run actually enforces the timeout rather than
// hanging forever on an unresponsive plugin.
func TestPlugin_Run_Timeout(t *testing.T) {
	p, err := load(t, "timeout")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	findings := p.Run(core.FileContext{Path: "f.go"})
	if findings != nil {
		t.Errorf("Run: got %v findings, want nil", findings)
	}
	if p.alive {
		t.Error("a timed-out plugin should be abandoned")
	}
}

func TestPlugin_Configure(t *testing.T) {
	p, err := load(t, "ok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	if err := p.Configure(map[string]string{"key": "value"}); err != nil {
		t.Errorf("Configure: %v", err)
	}
	if !p.alive {
		t.Error("Configure should not abandon a healthy plugin")
	}
}

func TestPlugin_Configure_NoopWhenNotAlive(t *testing.T) {
	p, err := load(t, "ok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p.Close()

	if err := p.Configure(map[string]string{"key": "value"}); err != nil {
		t.Errorf("Configure on a closed plugin: got %v, want nil (silent no-op)", err)
	}
}

func TestPlugin_Close_IsIdempotent(t *testing.T) {
	p, err := load(t, "ok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p.Close()
	p.Close() // must not panic or hang
}
