// Command winapisource fetches and verifies pinned official inputs.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/zzuf/GoWin-AFO/generator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "winapisource:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: winapisource <fetch|verify|list|update --dry-run>")
	}
	m, err := generator.OpenSourceManager("")
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch args[0] {
	case "fetch":
		results, e := m.Fetch(ctx)
		printJSON(results)
		return e
	case "verify":
		results, e := m.Verify()
		printJSON(results)
		return e
	case "list":
		for _, s := range m.List() {
			fmt.Printf("%s\t%s\t%s\t%s\n", s.ID, s.Type, s.Version, s.SHA256)
		}
		return nil
	case "update":
		fs := flag.NewFlagSet("update", flag.ContinueOnError)
		dry := fs.Bool("dry-run", false, "report updates without modifying the lock")
		if err = fs.Parse(args[1:]); err != nil {
			return err
		}
		if !*dry {
			return errors.New("update requires --dry-run; lock updates are review-only")
		}
		updates, e := m.DryRunUpdates(ctx)
		printJSON(updates)
		return e
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printJSON(v any) { enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); _ = enc.Encode(v) }
