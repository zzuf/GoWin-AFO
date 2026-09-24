package source

import "testing"

// A package's WinMD can be nested; selecting a hard-coded cache-root filename
// bypasses the exact archive member pinned by the source lock.
func TestMetadataFileSelectsOnlyLockedWinMD(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  string
	}{
		{"root", []string{"Windows.Win32.winmd", "sdk_license.txt"}, "Windows.Win32.winmd"},
		{"nested", []string{"License.txt", "c/UnionMetadata/10.0.26100.0/Windows.winmd"}, "c/UnionMetadata/10.0.26100.0/Windows.winmd"},
		{"missing", []string{"License.txt"}, ""},
		{"ambiguous", []string{"Windows.winmd", "facade/Windows.winmd"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (Locked{ID: "test-sdk", Files: tc.files}).MetadataFile()
			if tc.want == "" {
				if err == nil {
					t.Fatalf("selected %q without one unambiguous locked WinMD", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
