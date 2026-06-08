package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_Valid(t *testing.T) {
	c, err := LoadFile("testdata/valid.yml", true)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got, want := c.Global.DefaultModule, "default"; got != want {
		t.Errorf("default_module = %q, want %q", got, want)
	}
	mod, ok := c.Module("default")
	if !ok {
		t.Fatal("default module missing")
	}
	if mod.Collectors["apoc_kernel"] != Auto {
		t.Errorf("apoc_kernel = %v, want Auto", mod.Collectors["apoc_kernel"])
	}
	if mod.Collectors["jolokia"] != Enabled {
		t.Errorf("jolokia = %v, want Enabled", mod.Collectors["jolokia"])
	}
	if len(c.Targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(c.Targets))
	}
	tgt := c.Targets[0]
	if tgt.Address != "localhost:7687" {
		t.Errorf("address = %q", tgt.Address)
	}
	if tgt.Bolt.Scheme != "neo4j" {
		t.Errorf("scheme = %q", tgt.Bolt.Scheme)
	}
}

func TestFindTarget(t *testing.T) {
	c, err := LoadFile("testdata/valid.yml", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindTarget("localhost:7687"); !ok {
		t.Error("FindTarget by address failed")
	}
	if _, ok := c.FindTarget("local"); !ok {
		t.Error("FindTarget by name failed")
	}
	if _, ok := c.FindTarget("nope:7687"); ok {
		t.Error("FindTarget should miss unknown address (no default_target)")
	}
}

func TestEnableModeUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want EnableMode
	}{
		{"true", Enabled}, {"false", Disabled}, {"auto", Auto},
		{"yes", Enabled}, {"no", Disabled}, {"on", Enabled}, {"off", Disabled},
	}
	for _, tc := range cases {
		var m EnableMode
		if err := unmarshalScalar(tc.in, &m); err != nil {
			t.Errorf("%q: %v", tc.in, err)
		}
		if m != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, m, tc.want)
		}
	}
}

func TestExpandSecrets(t *testing.T) {
	t.Setenv("FOO", "bar")
	dir := t.TempDir()
	pwfile := filepath.Join(dir, "pw")
	if err := os.WriteFile(pwfile, []byte("supersecret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in := "username: ${FOO}\npassword: ${file:" + pwfile + "}\n"
	out, err := expandSecrets(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "username: bar\npassword: supersecret\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

// Helper: yaml.v3 doesn't expose a way to unmarshal a scalar string directly
// to a custom type without a wrapping doc. We use a tiny wrapper.
func unmarshalScalar(s string, m *EnableMode) error {
	type w struct {
		V EnableMode `yaml:"v"`
	}
	var got w
	return yamlUnmarshal([]byte("v: "+s+"\n"), &got, m)
}

func TestCheckSecretFileMode(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		mode    os.FileMode
		wantErr bool
	}{
		{0o400, false}, // owner read only
		{0o440, false}, // K8s fsGroup applied to 0400
		{0o600, false}, // owner read+write
		{0o640, false}, // K8s fsGroup applied to 0600
		{0o444, true},  // world-readable
		{0o644, true},  // world-readable (Secret default without defaultMode)
		{0o660, true},  // group-write
		{0o500, true},  // execute bits
		{0o700, true},  // execute bits
		{0o000, true},  // owner has no read
	}
	for _, tc := range cases {
		path := filepath.Join(dir, fmt.Sprintf("mode-%o", tc.mode))
		if err := os.WriteFile(path, []byte("x"), tc.mode); err != nil {
			t.Fatal(err)
		}
		// WriteFile applies umask; reapply mode explicitly so the test is deterministic.
		if err := os.Chmod(path, tc.mode); err != nil {
			t.Fatal(err)
		}
		err := checkSecretFileMode(path, false)
		if (err != nil) != tc.wantErr {
			t.Errorf("mode %o: got err=%v, wantErr=%v", tc.mode, err, tc.wantErr)
		}
		if tc.wantErr {
			// allow-insecure must always pass
			if err := checkSecretFileMode(path, true); err != nil {
				t.Errorf("mode %o with allowInsecure=true: unexpected err %v", tc.mode, err)
			}
		}
	}
}
