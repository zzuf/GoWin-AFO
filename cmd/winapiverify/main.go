// Command winapiverify validates generated artifacts and independently measured
// Windows ABI facts against the normalized binding IR.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go-windows-api.local/generator"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "winapiverify:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: winapiverify abi --all")
	}
	if args[0] != "abi" {
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("abi", flag.ContinueOnError)
	all := fs.Bool("all", false, "validate all checked-in evidence for the vertical slice (not all SDK namespaces)")
	root := fs.String("root", "", "project root (normally auto-detected)")
	output := fs.String("output", filepath.Join("coverage", "abi-latest.json"), "machine-readable report output")
	manifest := fs.String("slice-manifest", filepath.Join("bindings", "slice.manifest.json"), "generated slice manifest")
	lock := fs.String("source-lock", "sources.lock.json", "pinned source lock")
	probe := fs.String("probe-manifest", filepath.Join("tools", "abi-oracle", "testdata", "probe-manifest.json"), "reviewed ABI probe manifest")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !*all {
		return errors.New("abi verification requires --all so architecture limitations are reported explicitly")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	report, err := generator.VerifyABI(context.Background(), generator.ABIVerifyOptions{
		Root: *root, Output: *output, SliceManifest: *manifest,
		SourceLock: *lock, ProbeManifest: *probe,
	})
	fmt.Fprintf(stdout, "ABI verification: scope=%s passed=%t facts=%d matched=%d mismatched=%d symbols=%d fully-verified=%d\n",
		report.Scope.Name,
		report.Passed, report.Summary.ABIFactCount, report.Summary.MatchedABIFactCount,
		report.Summary.MismatchedABIFactCount, report.Summary.MatchedSymbolCount,
		report.Summary.VerifiedSymbolCount)
	return err
}
