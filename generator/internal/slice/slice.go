// Package slice reconstructs the reviewed vertical-slice bindings from pinned
// templates. These hand-reviewed projections are separate from the general
// metadata emitter and do not contribute to official projection coverage.
package slice

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed templates/*.tmpl
var templates embed.FS

type Tree struct {
	Files map[string][]byte
}

type sourcePin struct {
	ID      string
	Version string
	SHA256  string
}

type descriptor struct {
	SourceID string
	Name     string
}

// Changes to these pins require reviewing the slice against the new official
// metadata and ABI evidence before updating the templates.
var pins = []sourcePin{
	{"microsoft-win32metadata", "71.0.26-preview", "758efe32666596b8596c58a8c46631da23757a914713b4e88403bbfa5110cb38"},
	{"microsoft-wdkmetadata", "0.13.25-experimental", "79079a449d8f621052bcaf13039e411d1c1b77aec4a89cf04d96518d0a619617"},
	{"windows-sdk-winrt", "10.0.26100.0", "e2dee80d011cb9fc1276a0bd9f244f7a58d5ca72fe906a56e90d61c68cf8601a"},
}

var descriptors = []descriptor{
	{"microsoft-win32metadata", "e2e-slice-win32-foundation-v1"},
	{"microsoft-win32metadata", "e2e-slice-win32-kernel32-v1"},
	{"microsoft-wdkmetadata", "e2e-slice-wdk-nt-v1"},
	{"windows-sdk-winrt", "e2e-slice-winrt-windows-foundation-v1"},
}

var artifactPaths = []string{
	"bindings/wdk/nt/zavailability_kernel.go",
	"bindings/wdk/nt/ztypes_nt.go",
	"bindings/win32/foundation/zavailability_types.go",
	"bindings/win32/foundation/ztypes_base.go",
	"bindings/win32/foundation/ztypes_union_386.go",
	"bindings/win32/foundation/ztypes_union_64.go",
	"bindings/win32/kernel32/zavailability_kernel.go",
	"bindings/win32/kernel32/zcallbacks_locale.go",
	"bindings/win32/kernel32/zconstants_memory.go",
	"bindings/win32/kernel32/zfunctions_kernel_windows.go",
	"bindings/win32/kernel32/zfunctions_qpc_windows_386.go",
	"bindings/win32/kernel32/zfunctions_qpc_windows_64.go",
	"bindings/winrt/windows/foundation/zclasses_uri.go",
}

type manifest struct {
	SchemaVersion int        `json:"schemaVersion"`
	Generator     string     `json:"generator"`
	Inputs        []input    `json:"inputs"`
	Artifacts     []artifact `json:"artifacts"`
}
type input struct {
	SourceID              string `json:"sourceId"`
	Version               string `json:"version"`
	SHA256                string `json:"sha256"`
	SliceDescriptor       string `json:"sliceDescriptor"`
	SliceDescriptorSHA256 string `json:"sliceDescriptorSha256"`
}
type artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Render rebuilds all slice outputs without reading any existing generated
// artifact. The source lock is checked before rendering so stale templates
// cannot silently claim provenance from a newer input.
func Render(root, modulePath, version string) (Tree, error) {
	if modulePath == "" || strings.ContainsAny(modulePath, "\r\n\t \"'`\\{}") || strings.HasSuffix(modulePath, "/") || version == "" {
		return Tree{}, fmt.Errorf("invalid module path or generator version")
	}
	if err := verifyPins(filepath.Join(root, "sources.lock.json")); err != nil {
		return Tree{}, err
	}
	tree := Tree{Files: make(map[string][]byte, len(artifactPaths)+1)}
	m := manifest{SchemaVersion: 1, Generator: "winapigen v" + version, Inputs: make([]input, 0, len(descriptors)), Artifacts: make([]artifact, 0, len(artifactPaths))}
	for _, d := range descriptors {
		pin := pinFor(d.SourceID)
		m.Inputs = append(m.Inputs, input{pin.ID, pin.Version, pin.SHA256, d.Name, sha(d.Name)})
	}
	for _, path := range artifactPaths {
		name := "templates/" + strings.ReplaceAll(path, "/", "_") + ".tmpl"
		body, err := fs.ReadFile(templates, name)
		if err != nil {
			return Tree{}, fmt.Errorf("read reviewed slice template %s: %w", name, err)
		}
		body = bytes.ReplaceAll(body, []byte("{{MODULE}}"), []byte(modulePath))
		body = bytes.ReplaceAll(body, []byte("{{VERSION}}"), []byte(version))
		body = bytes.ReplaceAll(body, []byte("{{WIN32_VERSION}}"), []byte(pinFor("microsoft-win32metadata").Version))
		if bytes.Contains(body, []byte("{{")) {
			return Tree{}, fmt.Errorf("unresolved placeholder in %s", name)
		}
		body, err = format.Source(body)
		if err != nil {
			return Tree{}, fmt.Errorf("gofmt reviewed slice %s: %w", name, err)
		}
		tree.Files[path] = body
		m.Artifacts = append(m.Artifacts, artifact{path, shaBytes(body)})
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Tree{}, err
	}
	tree.Files["bindings/slice.manifest.json"] = append(b, '\n')
	return tree, nil
}

func verifyPins(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read source lock: %w", err)
	}
	var lock struct {
		Sources []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return fmt.Errorf("parse source lock: %w", err)
	}
	for _, pin := range pins {
		found := false
		for _, source := range lock.Sources {
			if source.ID == pin.ID {
				found = true
				if source.Version != pin.Version || source.SHA256 != pin.SHA256 {
					return fmt.Errorf("reviewed slice pin %s differs from source lock; update templates only after source and ABI review", pin.ID)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("reviewed slice pin %s missing from source lock", pin.ID)
		}
	}
	return nil
}

func pinFor(id string) sourcePin {
	for _, pin := range pins {
		if pin.ID == id {
			return pin
		}
	}
	panic("unregistered slice source: " + id)
}

func sha(s string) string { return shaBytes([]byte(s)) }
func shaBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
