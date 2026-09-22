// Command abi-oracle generates and compares Windows SDK ABI probes.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zzuf/GoWin-AFO/tools/abi-oracle/internal/oracle"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "abi-oracle:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "generate":
		return generate(args[1:])
	case "validate":
		return validate(args[1:])
	case "compare":
		return compare(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func generate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	manifestPath := fs.String("manifest", "", "path to the probe manifest")
	outPath := fs.String("out", "", "output C++ path, or - for stdout")
	architecture := fs.String("architecture", "", "override target: 386, amd64, arm64, or arm64ec")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" || *outPath == "" {
		return errors.New("generate requires --manifest and --out")
	}
	manifest, err := oracle.LoadManifest(*manifestPath)
	if err != nil {
		return err
	}
	if *architecture != "" {
		manifest.Architecture = *architecture
	}
	source, err := oracle.Generate(manifest)
	if err != nil {
		return err
	}
	return writeOutput(*outPath, source)
}

func validate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	manifestPath := fs.String("manifest", "", "manifest to validate")
	resultPath := fs.String("result", "", "oracle result to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if (*manifestPath == "") == (*resultPath == "") {
		return errors.New("validate requires exactly one of --manifest or --result")
	}
	if *manifestPath != "" {
		_, err := oracle.LoadManifest(*manifestPath)
		return err
	}
	_, err := oracle.LoadResult(*resultPath)
	return err
}

func compare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	expectedPath := fs.String("expected", "", "checked-in expected result")
	actualPath := fs.String("actual", "", "newly measured result")
	outPath := fs.String("out", "-", "comparison JSON path, or - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *expectedPath == "" || *actualPath == "" {
		return errors.New("compare requires --expected and --actual")
	}
	expected, err := oracle.LoadResult(*expectedPath)
	if err != nil {
		return fmt.Errorf("expected: %w", err)
	}
	actual, err := oracle.LoadResult(*actualPath)
	if err != nil {
		return fmt.Errorf("actual: %w", err)
	}
	comparison := oracle.Compare(expected, actual)
	b, err := oracle.MarshalComparison(comparison)
	if err != nil {
		return err
	}
	if err := writeOutput(*outPath, b); err != nil {
		return err
	}
	if !comparison.Equal {
		return fmt.Errorf("ABI mismatch: %d difference(s)", len(comparison.Mismatches))
	}
	return nil
}

func writeOutput(path string, contents []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(contents)
		return err
	}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, contents) {
		return nil
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(contents); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s atomically: %w", path, err)
	}
	keep = true
	return nil
}

func usageError() error { return errors.New(usage) }

const usage = `Usage:
  go run ./tools/abi-oracle generate --manifest FILE --out PROBE.cpp [--architecture ARCH]
  go run ./tools/abi-oracle validate (--manifest FILE | --result FILE)
  go run ./tools/abi-oracle compare --expected FILE --actual FILE [--out DIFF.json]
`
