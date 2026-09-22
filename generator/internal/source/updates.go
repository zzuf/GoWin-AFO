package source

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const nugetBase = "https://api.nuget.org/v3-flatcontainer/"

// Validate the provenance, not just the bytes: an artifact's hash alone cannot
// detect a lock entry that names one package while retrieving another.
func validateNuGetLocator(s Locked) error {
	u, err := url.Parse(s.Retrieval)
	if err != nil {
		return err
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return errors.New("NuGet retrieval must be an exact package content URL")
	}
	parts := strings.Split(strings.TrimPrefix(s.Retrieval, nugetBase), "/")
	if len(parts) != 3 || parts[0] != strings.ToLower(s.Package) || parts[2] != parts[0]+"."+parts[1]+".nupkg" {
		return errors.New("NuGet retrieval does not match locked package identity")
	}
	version, err := parseNuGetVersion(parts[1])
	if err != nil {
		return err
	}
	locked, err := parseNuGetVersion(s.Version)
	if err != nil {
		return err
	}
	if version.compare(locked) != 0 || strings.Contains(parts[1], "+") {
		return errors.New("NuGet retrieval does not match locked package version")
	}
	return nil
}

// readNuGetUpdate deliberately retains a stable pin's release channel. A
// prerelease pin may advance to either a prerelease or stable version. The
// content index includes unlisted versions, so this is discovery, not approval
// to change the lock or a claim about support/lifecycle status.
func readNuGetUpdate(resp *http.Response, current string) (string, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("NuGet version index: HTTP %d", resp.StatusCode)
	}
	const limit = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", err
	}
	if len(data) > limit {
		return "", errors.New("NuGet version index exceeds 4 MiB limit")
	}
	var body struct {
		Versions []string `json:"versions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err = decoder.Decode(&body); err != nil {
		return "", fmt.Errorf("NuGet version index: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return "", errors.New("NuGet version index contains trailing data")
	}
	if len(body.Versions) == 0 {
		return "", errors.New("NuGet version index has no versions")
	}
	base, err := parseNuGetVersion(current)
	if err != nil {
		return "", fmt.Errorf("current version: %w", err)
	}
	best, latest := base, current
	for _, raw := range body.Versions {
		v, err := parseNuGetVersion(raw)
		if err != nil {
			return "", fmt.Errorf("NuGet version index: %w", err)
		}
		if len(base.pre) == 0 && len(v.pre) != 0 {
			continue
		}
		if v.compare(best) > 0 {
			best, latest = v, raw
		}
	}
	return latest, nil
}

// NuGet version precedence differs from SemVer in allowing 1-4 numeric parts
// and case-insensitive prerelease labels. Build metadata has no precedence.
// See https://learn.microsoft.com/nuget/concepts/package-versioning.
type nugetVersion struct {
	parts [4]uint64
	pre   []string
}

func parseNuGetVersion(raw string) (nugetVersion, error) {
	var v nugetVersion
	invalid := fmt.Errorf("invalid NuGet version %q", raw)
	if len(raw) == 0 || len(raw) > 1024 {
		return v, invalid
	}
	version, metadata, hasMetadata := strings.Cut(raw, "+")
	if hasMetadata && !validVersionLabels(metadata) {
		return v, invalid
	}
	numeric, prerelease, hasPrerelease := strings.Cut(version, "-")
	if hasPrerelease {
		if !validVersionLabels(prerelease) {
			return v, invalid
		}
		v.pre = strings.Split(strings.ToLower(prerelease), ".")
	}
	parts := strings.Split(numeric, ".")
	if len(parts) > 4 {
		return v, invalid
	}
	for i, part := range parts {
		if !isDecimal(part) {
			return v, invalid
		}
		n, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return v, invalid
		}
		v.parts[i] = n
	}
	return v, nil
}

func validVersionLabels(s string) bool {
	for _, label := range strings.Split(s, ".") {
		if label == "" {
			return false
		}
		for _, ch := range label {
			if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '-') {
				return false
			}
		}
	}
	return true
}

func isDecimal(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (v nugetVersion) compare(other nugetVersion) int {
	for i, part := range v.parts {
		if c := cmp.Compare(part, other.parts[i]); c != 0 {
			return c
		}
	}
	if len(v.pre) == 0 || len(other.pre) == 0 {
		if len(v.pre) == len(other.pre) {
			return 0
		}
		if len(v.pre) == 0 {
			return 1
		}
		return -1
	}
	for i := 0; i < len(v.pre) && i < len(other.pre); i++ {
		a, b := v.pre[i], other.pre[i]
		an, bn := isDecimal(a), isDecimal(b)
		if an != bn {
			if an {
				return -1
			}
			return 1
		}
		if an {
			a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
			if c := cmp.Compare(len(a), len(b)); c != 0 {
				return c
			}
		}
		if c := strings.Compare(a, b); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(v.pre), len(other.pre))
}
