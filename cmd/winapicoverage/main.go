// Command winapicoverage reports and gates projection accounting.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zzuf/GoWin-AFO/generator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "winapicoverage:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: winapicoverage <report|diff|check|baseline>")
	}
	switch args[0] {
	case "report":
		return report(args[1:])
	case "diff":
		return diff(args[1:])
	case "check":
		return check(args[1:])
	case "baseline":
		return baseline(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func baseline(args []string) error {
	fs := flag.NewFlagSet("baseline", flag.ContinueOnError)
	accept := fs.Bool("accept", false, "explicitly accept the current report as baseline")
	input := fs.String("input", filepath.Join("coverage", "latest.json"), "current coverage")
	output := fs.String("output", filepath.Join("coverage", "baseline.json"), "baseline output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*accept {
		return errors.New("baseline requires --accept")
	}
	r, err := generator.ReadCoverage(*input)
	if err != nil {
		return err
	}
	if err = generator.WriteCoverage(*output, r); err != nil {
		return err
	}
	fmt.Printf("accepted baseline manifest=%s symbols=%d\n", r.ManifestHash, r.TotalSymbols)
	return nil
}
func report(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	format := fs.String("format", "markdown", "markdown or json")
	path := fs.String("input", filepath.Join("coverage", "latest.json"), "coverage report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := generator.ReadCoverage(*path)
	if err != nil {
		return err
	}
	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if *format != "markdown" {
		return fmt.Errorf("unknown format %q", *format)
	}
	fmt.Print(generator.CoverageMarkdown(r))
	return nil
}
func diff(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: winapicoverage diff <before.json> <after.json>")
	}
	a, err := generator.ReadCoverage(args[0])
	if err != nil {
		return err
	}
	b, err := generator.ReadCoverage(args[1])
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(generator.DiffCoverage(a, b))
}
func check(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	failU := fs.Bool("fail-unclassified", false, "fail if any symbol is unclassified")
	failR := fs.Bool("fail-regression", false, "compare against baseline")
	latest := fs.String("input", filepath.Join("coverage", "latest.json"), "latest coverage")
	baselinePath := fs.String("baseline", filepath.Join("coverage", "baseline.json"), "coverage baseline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := generator.ReadCoverage(*latest)
	if err != nil {
		return err
	}
	var baseline *generator.CoverageReport
	if *failR {
		b, e := generator.ReadCoverage(*baselinePath)
		if e != nil {
			return e
		}
		baseline = &b
	}
	if err = generator.CheckCoverage(r, baseline, *failU, *failR); err != nil {
		return err
	}
	fmt.Printf("projection accounting coverage: %.2f%% (%d/%d); unclassified=%d; ABI mismatches=%d\n", r.Metrics.ProjectionAccountingCoverage.Percent, r.Metrics.ProjectionAccountingCoverage.Numerator, r.Metrics.ProjectionAccountingCoverage.Denominator, r.ByStatus["unclassified"], r.ABIMismatches)
	return nil
}
