package generator

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	internalcoverage "github.com/zzuf/GoWin-AFO/generator/internal/coverage"
	"github.com/zzuf/GoWin-AFO/generator/internal/emit"
	"github.com/zzuf/GoWin-AFO/generator/internal/model"
	"github.com/zzuf/GoWin-AFO/generator/internal/slice"
)

type generatedTarget struct {
	path string
	dir  bool
}

// writeGeneration stages all rendered products before touching the checked-in
// tree. A failed replacement restores every destination from the stage backup.
func writeGeneration(root string, inv model.Inventory, report internalcoverage.Report, raw emit.Tree, reviewed slice.Tree, rawDumps map[string][]byte) error {
	stage, err := os.MkdirTemp(root, ".winapigen-stage-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := emit.Write(stage, raw); err != nil {
		return err
	}
	if err := slice.Write(stage, reviewed); err != nil {
		return err
	}
	if err := writeReports(stage, inv, report); err != nil {
		return err
	}
	for path, data := range rawDumps {
		if !safeRawDumpPath(path) {
			return fmt.Errorf("unsafe raw dump path %q", path)
		}
		file := filepath.Join(stage, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	targets := []generatedTarget{}
	for _, path := range []string{"bindings/generated", "bridge/generated"} {
		if _, err := os.Stat(filepath.Join(stage, filepath.FromSlash(path))); err == nil {
			targets = append(targets, generatedTarget{path: path, dir: true})
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	paths := make([]string, 0, len(reviewed.Files))
	for path := range reviewed.Files {
		if path != "bindings/slice.manifest.json" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	paths = append(paths, "bindings/slice.manifest.json")
	for _, path := range paths {
		targets = append(targets, generatedTarget{path: path})
	}
	for _, path := range []string{"coverage/inventory.json.gz", "coverage/latest.json", "coverage/latest.md"} {
		targets = append(targets, generatedTarget{path: path})
	}
	rawPaths := make([]string, 0, len(rawDumps))
	for path := range rawDumps {
		rawPaths = append(rawPaths, path)
	}
	sort.Strings(rawPaths)
	for _, path := range rawPaths {
		targets = append(targets, generatedTarget{path: path})
	}
	if err := preflightGeneration(root, stage, targets, reviewed); err != nil {
		return err
	}
	if err := commitGeneration(root, stage, targets); err != nil {
		var retained *retainedStageError
		if errors.As(err, &retained) {
			cleanup = false
		}
		return err
	}
	return nil
}

func safeRawDumpPath(path string) bool {
	if !strings.HasPrefix(path, "coverage/raw/") || !strings.HasSuffix(path, ".json") {
		return false
	}
	base := strings.TrimPrefix(path, "coverage/raw/")
	return base != "" && !strings.ContainsAny(base, `/\`) && filepath.ToSlash(filepath.Clean(path)) == path
}

func preflightGeneration(root, stage string, targets []generatedTarget, reviewed slice.Tree) error {
	if err := slice.ValidateDestinations(root, reviewed); err != nil {
		return err
	}
	for _, target := range targets {
		if !strings.HasPrefix(target.path, "bindings/") && !strings.HasPrefix(target.path, "bridge/") && !strings.HasPrefix(target.path, "coverage/") {
			return fmt.Errorf("unexpected generation target %q", target.path)
		}
		source := filepath.Join(stage, filepath.FromSlash(target.path))
		info, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("missing staged output %s: %w", target.path, err)
		}
		if info.IsDir() != target.dir {
			return fmt.Errorf("staged output has wrong kind: %s", target.path)
		}
		dest := filepath.Join(root, filepath.FromSlash(target.path))
		info, err = os.Lstat(dest)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if target.dir {
			if !info.IsDir() {
				return fmt.Errorf("generated tree collision: %s", dest)
			}
			marker := filepath.Join(dest, ".winapigen.json")
			if info, err := os.Lstat(marker); err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("refusing to replace unowned generated tree %s", dest)
			}
			if err := validateOwnedGeneratedTree(dest); err != nil {
				return err
			}
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("generated file collision: %s", dest)
		} else if strings.HasPrefix(target.path, "coverage/") {
			if err := validateOwnedReport(dest, target.path); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOwnedReport(dest, path string) error {
	if path == "coverage/inventory.json.gz" {
		file, err := os.Open(dest)
		if err != nil {
			return err
		}
		defer file.Close()
		reader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("refusing to replace unrecognized inventory report %s: %w", dest, err)
		}
		defer reader.Close()
		prefix := make([]byte, 256)
		n, err := io.ReadFull(reader, prefix)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("refusing to replace unreadable inventory report %s: %w", dest, err)
		}
		if !bytes.HasPrefix(prefix[:n], []byte(`{"schemaVersion":1,"generatorVersion":"`)) {
			return fmt.Errorf("refusing to replace unrecognized inventory report %s", dest)
		}
		return nil
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	switch path {
	case "coverage/latest.json":
		var report struct {
			SchemaVersion int    `json:"schemaVersion"`
			Scope         string `json:"scope"`
		}
		if err := json.Unmarshal(data, &report); err == nil && report.SchemaVersion == 1 && (report.Scope == "official-windows-sources" || report.Scope == "vertical-slice-fixture") {
			return nil
		}
	case "coverage/latest.md":
		if bytes.HasPrefix(data, []byte("# Coverage report\n")) && bytes.Contains(data, []byte("\nScope: `")) {
			return nil
		}
	default:
		if safeRawDumpPath(path) && json.Valid(data) {
			return nil
		}
	}
	return fmt.Errorf("refusing to replace unrecognized generated report %s", dest)
}

func validateOwnedGeneratedTree(dest string) error {
	data, err := os.ReadFile(filepath.Join(dest, ".winapigen.json"))
	if err != nil {
		return err
	}
	var marker struct {
		SchemaVersion int      `json:"schemaVersion"`
		Files         []string `json:"files"`
	}
	if err := json.Unmarshal(data, &marker); err != nil || marker.SchemaVersion != 1 || len(marker.Files) == 0 {
		return fmt.Errorf("invalid generated-tree ownership marker in %s", dest)
	}
	allowed := map[string]bool{}
	for _, path := range marker.Files {
		if path == "" || path == "." || strings.HasPrefix(path, "../") || strings.Contains(path, "\\") || filepath.ToSlash(filepath.Clean(path)) != path || !fs.ValidPath(path) {
			return fmt.Errorf("unsafe path %q in generated-tree marker %s", path, dest)
		}
		allowed[path] = true
	}
	return filepath.WalkDir(dest, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dest {
			return nil
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink inside generated tree: %s", path)
		}
		if entry.IsDir() {
			for owned := range allowed {
				if strings.HasPrefix(owned, rel+"/") {
					return nil
				}
			}
			return fmt.Errorf("unknown directory inside generated tree: %s", path)
		}
		if rel == ".winapigen.json" {
			return nil
		}
		if !entry.Type().IsRegular() || !allowed[rel] {
			return fmt.Errorf("unknown file inside generated tree: %s", path)
		}
		file, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(file, []byte("// Code generated by winapigen ")) && !(bytes.HasPrefix(file, []byte("//go:build ")) && bytes.Contains(file[:min(len(file), 256)], []byte("// Code generated by winapigen "))) {
			return fmt.Errorf("handwritten file inside generated tree: %s", path)
		}
		return nil
	})
}

type retainedStageError struct{ path string }

func (e *retainedStageError) Error() string {
	return "rollback incomplete; backups retained at " + e.path
}

type transactionOps struct {
	rename    func(string, string) error
	remove    func(string) error
	removeAll func(string) error
}

func commitGeneration(root, stage string, targets []generatedTarget) error {
	return commitGenerationWithOps(root, stage, targets, transactionOps{os.Rename, os.Remove, os.RemoveAll})
}

func commitGenerationWithOps(root, stage string, targets []generatedTarget, ops transactionOps) error {
	type replacement struct {
		generatedTarget
		dest, backup string
		installed    bool
	}
	var changed []replacement
	rollback := func(cause error) error {
		var restoreErrors []error
		for i := len(changed) - 1; i >= 0; i-- {
			item := changed[i]
			if item.installed {
				var err error
				if item.dir {
					err = ops.removeAll(item.dest)
				} else {
					err = ops.remove(item.dest)
				}
				if err != nil {
					restoreErrors = append(restoreErrors, fmt.Errorf("remove installed %s: %w", item.dest, err))
					continue
				}
			}
			if item.backup != "" {
				if err := ops.rename(item.backup, item.dest); err != nil {
					restoreErrors = append(restoreErrors, fmt.Errorf("restore %s from %s: %w", item.dest, item.backup, err))
				}
			}
		}
		if len(restoreErrors) != 0 {
			restoreErrors = append(restoreErrors, &retainedStageError{path: stage})
		}
		return errors.Join(append([]error{cause}, restoreErrors...)...)
	}
	for i, target := range targets {
		dest := filepath.Join(root, filepath.FromSlash(target.path))
		source := filepath.Join(stage, filepath.FromSlash(target.path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return rollback(err)
		}
		backup := ""
		if _, err := os.Lstat(dest); err == nil {
			backup = filepath.Join(stage, "backups", fmt.Sprintf("%04d", i))
			if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
				return rollback(err)
			}
			if err := ops.rename(dest, backup); err != nil {
				return rollback(err)
			}
		} else if !os.IsNotExist(err) {
			return rollback(err)
		}
		changed = append(changed, replacement{generatedTarget: target, dest: dest, backup: backup})
		if err := ops.rename(source, dest); err != nil {
			return rollback(err)
		}
		changed[len(changed)-1].installed = true
	}
	return nil
}
