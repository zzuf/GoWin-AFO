package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// This catches selecting the final feed entry, downgrades, and inadvertent
// stable-to-preview updates. Expectations are literal NuGet precedence cases.
func TestDryRunUpdatesSelectsNewerCompatibleVersion(t *testing.T) {
	for _, tc := range []struct {
		name, current, versions, want string
		changed                       bool
	}{
		{"unordered", "1.0.0", `["1.9.0","1.10.0","1.2.0"]`, "1.10.0", true},
		{"stable channel", "2.3.1", `["2.5.1","3.0.0-preview"]`, "2.5.1", true},
		{"no downgrade", "2.0.0", `["1.0.0"]`, "2.0.0", false},
		{"numeric prerelease", "1.0.0-rc.1", `["1.0.0-rc.10","1.0.0-rc.2"]`, "1.0.0-rc.10", true},
		{"stable beats prerelease", "1.0.0-rc.1", `["1.0.0","1.0.0-rc.10"]`, "1.0.0", true},
		{"revision", "1.2.3", `["1.2.3.10","1.2.3.2"]`, "1.2.3.10", true},
		{"normalized equality", "1.0.0", `["1.0.0.0","1.0"]`, "1.0.0", false},
		{"case insensitive", "1.0.0-RC.1", `["1.0.0-rc.1"]`, "1.0.0-RC.1", false},
		{"metadata ignored", "1.0.0+build.1", `["1.0.0"]`, "1.0.0+build.1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := updateManager(t, tc.current, http.StatusOK, `{"versions":`+tc.versions+`}`)
			updates, err := m.DryRunUpdates(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(updates) != 1 || updates[0].Latest != tc.want || updates[0].Changed != tc.changed {
				t.Fatalf("got %+v; want latest=%s changed=%v", updates, tc.want, tc.changed)
			}
			if m.Lock.Sources[0].Version != tc.current {
				t.Fatal("dry run mutated source lock")
			}
		})
	}
}

func TestDryRunUpdatesRejectsMalformedFeed(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"versions":[]}`, `{"versions":["1.2.bad"]}`, `{"versions":["1.0.0-"]}`,
		`{"versions":["1.0.0+bad!"]}`, `{"versions":["1.0.0-rc..1"]}`,
		`{"versions":["1.0.0"]} {}`, `{"versions":["1.0.0"]}` + strings.Repeat(" ", 4<<20),
	} {
		m := updateManager(t, "1.0.0", http.StatusOK, body)
		updates, err := m.DryRunUpdates(context.Background())
		if err == nil {
			t.Fatalf("accepted malformed/oversized feed %.100q", body)
		}
		if len(updates) != 1 || updates[0].Changed || updates[0].Latest != "1.0.0" || updates[0].Message == "" {
			t.Fatalf("error was not accounted for: %+v", updates)
		}
	}
}

func TestDryRunUpdatesReportsHTTPFailure(t *testing.T) {
	m := updateManager(t, "1.0.0", http.StatusTooManyRequests, `{"versions":["2.0.0"]}`)
	updates, err := m.DryRunUpdates(context.Background())
	if err == nil || len(updates) != 1 || updates[0].Changed {
		t.Fatalf("got %+v, %v", updates, err)
	}
}

func TestDryRunUpdatesRejectsInvalidRequestURL(t *testing.T) {
	m := updateManager(t, "1.0.0", http.StatusOK, `{"versions":["2.0.0"]}`)
	m.Lock.Sources[0].Retrieval = "https://api.nuget.org/v3-flatcontainer/%zz/1.0.0/example.nupkg"
	updates, err := m.DryRunUpdates(context.Background())
	if err == nil || len(updates) != 1 || updates[0].Changed || updates[0].Message == "" {
		t.Fatalf("malformed request was not accounted for: %+v, %v", updates, err)
	}
}

func TestDryRunUpdatesRejectsMalformedNuGetLocator(t *testing.T) {
	m := updateManager(t, "1.0.0", http.StatusOK, `{"versions":["2.0.0"]}`)
	m.Lock.Sources[0].Retrieval = "https://api.nuget.org/v3-flatcontainer/example"
	updates, err := m.DryRunUpdates(context.Background())
	if err == nil || len(updates) != 1 || updates[0].Message == "" {
		t.Fatalf("got %+v, %v", updates, err)
	}
}

func updateManager(t *testing.T, current string, status int, body string) *Manager {
	t.Helper()
	return &Manager{
		Lock: Lock{Sources: []Locked{{ID: "example", Version: current, Retrieval: "https://api.nuget.org/v3-flatcontainer/example/1.0.0/example.1.0.0.nupkg"}}},
		Client: &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" || r.URL.String() != "https://api.nuget.org/v3-flatcontainer/example/index.json" {
				t.Fatalf("unexpected update request %s %s", r.Method, r.URL)
			}
			return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})},
	}
}
