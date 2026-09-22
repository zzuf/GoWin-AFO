package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresABIAll(t *testing.T) {
	for _, args := range [][]string{nil, {"abi"}, {"unknown"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%q) unexpectedly succeeded", args)
		}
	}
}

func TestRunABIAll(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "abi-latest.json")
	var stdout bytes.Buffer
	if err = run([]string{"abi", "--all", "--root", root, "--output", output}, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "scope=vertical-slice-fixture") || !strings.Contains(stdout.String(), "passed=true") || !strings.Contains(stdout.String(), "facts=94") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	b, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"passed": true`)) || bytes.Contains(b, []byte(root)) {
		t.Fatalf("unexpected report: %s", b)
	}
}
