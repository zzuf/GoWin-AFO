// Package source manages pinned, licensed upstream artifacts.
package source

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maxArtifactBytes int64 = 1 << 30

type Lock struct {
	SchemaVersion int      `json:"schemaVersion"`
	Sources       []Locked `json:"sources"`
}

type Locked struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Package           string   `json:"package"`
	Version           string   `json:"version"`
	Retrieval         string   `json:"retrieval"`
	SHA256            string   `json:"sha256"`
	LicenseIdentifier string   `json:"licenseIdentifier"`
	Architectures     []string `json:"architectures"`
	WindowsSDKVersion string   `json:"windowsSDKVersion"`
	Files             []string `json:"files"`
	DependsOn         []string `json:"dependsOn"`
	Required          bool     `json:"required"`
}

type Result struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type Manager struct {
	Root   string
	Lock   Lock
	Client *http.Client
}

func Open(root string) (*Manager, error) {
	if root == "" {
		var err error
		root, err = FindRoot()
		if err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "sources.lock.json"))
	if err != nil {
		return nil, err
	}
	var lock Lock
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&lock); err != nil {
		return nil, fmt.Errorf("decode sources.lock.json: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("decode sources.lock.json: trailing JSON value")
	}
	if err = validateLock(lock); err != nil {
		return nil, err
	}
	return &Manager{Root: root, Lock: lock, Client: &http.Client{Timeout: 10 * time.Minute}}, nil
}

var sourceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func validateLock(lock Lock) error {
	if lock.SchemaVersion != 1 {
		return fmt.Errorf("unsupported source lock schema %d", lock.SchemaVersion)
	}
	if len(lock.Sources) == 0 {
		return errors.New("source lock contains no sources")
	}
	byID := make(map[string]Locked, len(lock.Sources))
	for _, s := range lock.Sources {
		if !sourceIDPattern.MatchString(s.ID) || s.Type == "" || s.Package == "" || s.Version == "" || s.Retrieval == "" || s.LicenseIdentifier == "" || s.WindowsSDKVersion == "" || len(s.SHA256) != 64 || len(s.Architectures) == 0 || len(s.Files) == 0 {
			return fmt.Errorf("locked source %q lacks a required identity, version, retrieval, license, SDK, architecture, file, or hash field", s.ID)
		}
		if _, duplicate := byID[s.ID]; duplicate {
			return fmt.Errorf("duplicate source ID %q", s.ID)
		}
		byID[s.ID] = s
		if decoded, e := hex.DecodeString(s.SHA256); e != nil || len(decoded) != sha256.Size || s.SHA256 != strings.ToLower(s.SHA256) {
			return fmt.Errorf("source %s SHA-256 must be 64 lowercase hexadecimal digits", s.ID)
		}
		if !strings.HasPrefix(s.Retrieval, "https://") && !strings.HasPrefix(s.Retrieval, "windows-sdk://") {
			return fmt.Errorf("source %s uses an unsupported or insecure retrieval identifier", s.ID)
		}
		if strings.HasPrefix(s.Retrieval, nugetBase) {
			if err := validateNuGetLocator(s); err != nil {
				return fmt.Errorf("source %s: %w", s.ID, err)
			}
		}
		if strings.HasPrefix(s.Retrieval, "windows-sdk://") && len(s.Files) != 1 {
			return fmt.Errorf("source %s local Windows SDK locator must name exactly one file", s.ID)
		}
		architectures := map[string]bool{}
		for _, architecture := range s.Architectures {
			if architecture != "386" && architecture != "amd64" && architecture != "arm64" {
				return fmt.Errorf("source %s has unsupported architecture %q", s.ID, architecture)
			}
			if architectures[architecture] {
				return fmt.Errorf("source %s repeats architecture %q", s.ID, architecture)
			}
			architectures[architecture] = true
		}
		files := map[string]bool{}
		for _, name := range s.Files {
			clean := path.Clean(strings.ReplaceAll(name, `\`, "/"))
			if name == "" || clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
				return fmt.Errorf("source %s has unsafe locked file %q", s.ID, name)
			}
			if files[clean] {
				return fmt.Errorf("source %s repeats locked file %q", s.ID, name)
			}
			files[clean] = true
		}
		dependencies := map[string]bool{}
		for _, dependency := range s.DependsOn {
			if dependency == "" || dependency == s.ID || dependencies[dependency] {
				return fmt.Errorf("source %s has an empty, self, or duplicate dependency %q", s.ID, dependency)
			}
			dependencies[dependency] = true
			if strings.Contains(dependency, "/") {
				parts := strings.Split(dependency, "/")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					return fmt.Errorf("source %s has invalid pinned package dependency %q", s.ID, dependency)
				}
			}
		}
	}
	for _, s := range lock.Sources {
		for _, dependency := range s.DependsOn {
			if !strings.Contains(dependency, "/") {
				if _, ok := byID[dependency]; !ok {
					return fmt.Errorf("source %s depends on unknown source %q", s.ID, dependency)
				}
			}
		}
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("source dependency cycle includes %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, dependency := range byID[id].DependsOn {
			if !strings.Contains(dependency, "/") {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func FindRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err = os.Stat(filepath.Join(d, "sources.lock.json")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", errors.New("sources.lock.json not found in this directory or a parent")
}

func (m *Manager) CacheDir(s Locked) string { return filepath.Join(m.Root, "sources", "cache", s.ID) }

func (m *Manager) Fetch(ctx context.Context) ([]Result, error) {
	results := make([]Result, 0, len(m.Lock.Sources))
	var failures []error
	for _, s := range m.Lock.Sources {
		r, err := m.fetchOne(ctx, s)
		results = append(results, r)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", s.ID, err))
		}
	}
	return results, errors.Join(failures...)
}

func (m *Manager) fetchOne(ctx context.Context, s Locked) (Result, error) {
	dest := m.CacheDir(s)
	if ok, _ := m.verifyOne(s); ok {
		return Result{ID: s.ID, State: "verified", Path: dest}, nil
	}
	stage, err := os.MkdirTemp(filepath.Join(m.Root, "sources"), "staging-")
	if err != nil {
		return Result{ID: s.ID, State: "error", Message: err.Error()}, err
	}
	defer os.RemoveAll(stage)
	artifact := filepath.Join(stage, "artifact")
	switch {
	case strings.HasPrefix(s.Retrieval, "https://"):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Retrieval, nil)
		if err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
		resp, err := m.Client.Do(req)
		if err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return Result{ID: s.ID, State: "error", Message: resp.Status}, fmt.Errorf("download: %s", resp.Status)
		}
		f, err := os.OpenFile(artifact, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
		n, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxArtifactBytes+1))
		closeErr := f.Close()
		if copyErr != nil {
			return Result{ID: s.ID, State: "error", Message: copyErr.Error()}, copyErr
		}
		if closeErr != nil {
			return Result{ID: s.ID, State: "error", Message: closeErr.Error()}, closeErr
		}
		if n > maxArtifactBytes {
			return Result{ID: s.ID, State: "error", Message: "artifact exceeds size limit"}, errors.New("artifact exceeds 1 GiB size limit")
		}
	case strings.HasPrefix(s.Retrieval, "windows-sdk://"):
		path, err := resolveWindowsSDK(s)
		if err != nil {
			state := "external-sdk-not-installed"
			r := Result{ID: s.ID, State: state, Message: err.Error()}
			if s.Required {
				return r, err
			}
			return r, nil
		}
		if err = copyFile(path, artifact); err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
	default:
		return Result{ID: s.ID, State: "error", Message: "unsupported retrieval scheme"}, errors.New("unsupported retrieval scheme")
	}
	if err = verifyHash(artifact, s.SHA256); err != nil {
		return Result{ID: s.ID, State: "hash-mismatch", Message: err.Error()}, err
	}
	finalStage := filepath.Join(stage, "content")
	if err = os.Mkdir(finalStage, 0o755); err != nil {
		return Result{ID: s.ID, State: "error", Message: err.Error()}, err
	}
	if strings.HasPrefix(s.Retrieval, "https://") {
		if err = extractSelected(artifact, finalStage, s.Files); err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
		if err = copyFile(artifact, filepath.Join(finalStage, "artifact.nupkg")); err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
	} else {
		if err = copyFile(artifact, filepath.Join(finalStage, s.Files[0])); err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
		if err = copyFile(artifact, filepath.Join(finalStage, "artifact")); err != nil {
			return Result{ID: s.ID, State: "error", Message: err.Error()}, err
		}
	}
	manifest, _ := json.MarshalIndent(struct{ ID, Version, SHA256 string }{s.ID, s.Version, s.SHA256}, "", "  ")
	manifest = append(manifest, '\n')
	if err = os.WriteFile(filepath.Join(finalStage, "source.json"), manifest, 0o644); err != nil {
		return Result{ID: s.ID, State: "error", Message: err.Error()}, err
	}
	if err = installDir(finalStage, dest); err != nil {
		return Result{ID: s.ID, State: "error", Message: err.Error()}, err
	}
	return Result{ID: s.ID, State: "fetched", Path: dest}, nil
}

func resolveWindowsSDK(s Locked) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("Windows SDK local provider is only available on Windows")
	}
	root := os.Getenv("WindowsSdkDir")
	if root == "" {
		root = filepath.Join(os.Getenv("ProgramFiles(x86)"), "Windows Kits", "10")
	}
	path := filepath.Join(root, "UnionMetadata", s.WindowsSDKVersion, "Windows.winmd")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("Windows SDK %s: %w", s.WindowsSDKVersion, err)
	}
	return path, nil
}

func (m *Manager) Verify() ([]Result, error) {
	results := make([]Result, 0, len(m.Lock.Sources))
	var failures []error
	for _, s := range m.Lock.Sources {
		ok, err := m.verifyOne(s)
		if ok {
			results = append(results, Result{ID: s.ID, State: "verified", Path: m.CacheDir(s)})
			continue
		}
		state := "missing"
		if err != nil {
			state = "invalid"
		}
		results = append(results, Result{ID: s.ID, State: state, Message: errorString(err)})
		if err != nil || s.Required {
			if err == nil {
				err = errors.New("cache missing")
			}
			failures = append(failures, fmt.Errorf("%s: %w", s.ID, err))
		}
	}
	return results, errors.Join(failures...)
}

func (m *Manager) verifyOne(s Locked) (bool, error) {
	dir := m.CacheDir(s)
	artifact := filepath.Join(dir, "artifact.nupkg")
	if strings.HasPrefix(s.Retrieval, "windows-sdk://") {
		artifact = filepath.Join(dir, "artifact")
	}
	if _, err := os.Stat(artifact); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := verifyHash(artifact, s.SHA256); err != nil {
		return false, err
	}
	for _, name := range s.Files {
		path, err := safeJoin(dir, name)
		if err != nil {
			return false, err
		}
		if _, err = os.Stat(path); err != nil {
			return false, err
		}
	}
	if strings.HasPrefix(s.Retrieval, "https://") {
		if err := verifySelectedAgainstArchive(artifact, dir, s.Files); err != nil {
			return false, err
		}
	} else {
		selected := filepath.Join(dir, filepath.FromSlash(s.Files[0]))
		if err := verifyHash(selected, s.SHA256); err != nil {
			return false, fmt.Errorf("extracted SDK input differs from locked artifact: %w", err)
		}
	}
	var manifest struct{ ID, Version, SHA256 string }
	b, err := os.ReadFile(filepath.Join(dir, "source.json"))
	if err != nil {
		return false, err
	}
	if err = json.Unmarshal(b, &manifest); err != nil {
		return false, fmt.Errorf("cache source manifest: %w", err)
	}
	if manifest.ID != s.ID || manifest.Version != s.Version || !strings.EqualFold(manifest.SHA256, s.SHA256) {
		return false, errors.New("cache source manifest does not match lock")
	}
	return true, nil
}

func verifySelectedAgainstArchive(archive, dir string, names []string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open cached NuGet zip: %w", err)
	}
	defer z.Close()
	entries := map[string]*zip.File{}
	for _, entry := range z.File {
		entries[filepath.ToSlash(entry.Name)] = entry
	}
	for _, name := range names {
		entry := entries[filepath.ToSlash(name)]
		if entry == nil {
			return fmt.Errorf("locked archive entry %q is missing", name)
		}
		if entry.UncompressedSize64 > uint64(maxArtifactBytes) {
			return fmt.Errorf("locked archive entry %q exceeds size limit", name)
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		archiveHash := sha256.New()
		_, copyErr := io.Copy(archiveHash, io.LimitReader(input, maxArtifactBytes+1))
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		extracted, err := safeJoin(dir, name)
		if err != nil {
			return err
		}
		file, err := os.Open(extracted)
		if err != nil {
			return err
		}
		extractedHash := sha256.New()
		_, copyErr = io.Copy(extractedHash, file)
		closeErr = file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(archiveHash.Sum(nil), extractedHash.Sum(nil)) {
			return fmt.Errorf("extracted file %q differs from the hash-verified archive entry", name)
		}
	}
	return nil
}

func verifyHash(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA-256 mismatch: got %s want %s", got, want)
	}
	return nil
}

func extractSelected(archive, dest string, names []string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open NuGet zip: %w", err)
	}
	defer z.Close()
	wanted := map[string]bool{}
	for _, n := range names {
		n = filepath.ToSlash(n)
		if strings.Contains(n, "..") || strings.HasPrefix(n, "/") {
			return fmt.Errorf("unsafe locked filename %q", n)
		}
		wanted[n] = true
	}
	found := map[string]bool{}
	for _, entry := range z.File {
		name := filepath.ToSlash(entry.Name)
		if !wanted[name] {
			continue
		}
		if entry.UncompressedSize64 > uint64(maxArtifactBytes) {
			return fmt.Errorf("zip entry %s exceeds limit", name)
		}
		out, err := safeJoin(dest, name)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		src, err := entry.Open()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(f, io.LimitReader(src, int64(entry.UncompressedSize64)+1))
		closeErr := f.Close()
		srcErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if srcErr != nil {
			return srcErr
		}
		found[name] = true
	}
	for name := range wanted {
		if !found[name] {
			return fmt.Errorf("locked file %q absent from package", name)
		}
	}
	return nil
}

func safeJoin(root, name string) (string, error) {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	path := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", name)
	}
	return path, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func installDir(stage, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		backup := dest + ".old"
		if _, err = os.Stat(backup); err == nil {
			return fmt.Errorf("stale backup exists: %s", backup)
		}
		if err = os.Rename(dest, backup); err != nil {
			return err
		}
		if err = os.Rename(stage, dest); err != nil {
			_ = os.Rename(backup, dest)
			return err
		}
		return os.RemoveAll(backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(stage, dest)
}

func (m *Manager) List() []Locked {
	out := append([]Locked(nil), m.Lock.Sources...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type Update struct {
	ID, Current, Latest string
	Changed             bool
	Message             string
}

func (m *Manager) DryRunUpdates(ctx context.Context) ([]Update, error) {
	var updates []Update
	var failures []error
	for _, s := range m.Lock.Sources {
		u := Update{ID: s.ID, Current: s.Version, Latest: s.Version}
		if !strings.HasPrefix(s.Retrieval, "https://api.nuget.org/v3-flatcontainer/") {
			u.Message = "provider has no remote version index"
			updates = append(updates, u)
			continue
		}
		parts := strings.Split(strings.TrimPrefix(s.Retrieval, "https://api.nuget.org/v3-flatcontainer/"), "/")
		if len(parts) < 3 {
			u.Message = "unrecognized NuGet URL"
			failures = append(failures, fmt.Errorf("source %s: %s", s.ID, u.Message))
			updates = append(updates, u)
			continue
		}
		url := "https://api.nuget.org/v3-flatcontainer/" + parts[0] + "/index.json"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			failures = append(failures, err)
			u.Message = err.Error()
			updates = append(updates, u)
			continue
		}
		resp, err := m.Client.Do(req)
		if err != nil {
			failures = append(failures, err)
			u.Message = err.Error()
			updates = append(updates, u)
			continue
		}
		latest, err := readNuGetUpdate(resp, s.Version)
		if err != nil {
			failures = append(failures, err)
			u.Message = err.Error()
			updates = append(updates, u)
			continue
		}
		u.Latest = latest
		u.Changed = u.Latest != u.Current
		updates = append(updates, u)
	}
	return updates, errors.Join(failures...)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
