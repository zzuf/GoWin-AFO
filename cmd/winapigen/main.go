// Command winapigen ingests metadata, normalizes IR, and emits bindings.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go-windows-api.local/generator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "winapigen:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: winapigen <generate|inspect>")
	}
	switch args[0] {
	case "generate":
		return generate(args[1:])
	case "inspect":
		return inspect(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func generate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	all := fs.Bool("all", false, "ingest every available locked source")
	fixture := fs.Bool("fixture", false, "generate only the 23-symbol vertical-slice fixture")
	dumpRaw := fs.Bool("dump-raw", false, "write raw metadata debug dumps")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *all == *fixture {
		return errors.New("generate requires exactly one of --all or --fixture; refusing to replace outputs with an implicit scope")
	}
	start := time.Now()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	res, err := generator.Generate(context.Background(), generator.GenerateOptions{All: *all, DumpRaw: *dumpRaw, Write: true})
	if err != nil {
		return err
	}
	runtime.ReadMemStats(&after)
	excluded := 0
	for _, count := range res.Coverage.ExcludedBySource {
		excluded += count
	}
	fmt.Printf("manifest=%s inventory_records=%d coverage_scope=%s coverage_symbols=%d excluded_fixture=%d packages=%d files=%d elapsed=%s cumulative_alloc_delta=%d\n", res.Inventory.ManifestHash, len(res.Inventory.Symbols), res.Coverage.Scope, res.Coverage.TotalSymbols, excluded, res.Packages, res.Files, time.Since(start).Round(time.Millisecond), int64(after.TotalAlloc-before.TotalAlloc))
	return nil
}

func inspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	qualified := fs.String("symbol", "", "fully-qualified metadata name")
	native := fs.String("native", "", "native symbol name")
	id := fs.String("id", "", "stable symbol ID")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	inventory := fs.String("inventory", filepath.Join("coverage", "inventory.json.gz"), "inventory JSON or JSON.gz path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *qualified == "" && *native == "" && *id == "" {
		return errors.New("one of --symbol, --native, or --id is required")
	}
	inv, err := generator.ReadInventory(*inventory)
	if err != nil {
		res, e := generator.Generate(context.Background(), generator.GenerateOptions{Write: false})
		if e != nil {
			return fmt.Errorf("read inventory: %v; build fixture: %w", err, e)
		}
		inv = res.Inventory
	}
	found := generator.FindSymbols(inv, *qualified, *native, *id)
	if len(found) == 0 {
		return errors.New("symbol not found")
	}
	enc := json.NewEncoder(os.Stdout)
	if *asJSON || len(found) > 1 {
		enc.SetIndent("", "  ")
		return enc.Encode(found)
	}
	s := found[0]
	fmt.Printf("ID: %s\nName: %s\nKind: %s\nArchitecture: %s\nSignature: %s\nStatus: %s (%s)\nBackend: %s (%s)\n", s.ID, s.QualifiedName(), s.Kind, s.Architecture, s.CanonicalSignature, s.Status, s.StatusReason, s.Backend, s.BackendReason)
	return nil
}
