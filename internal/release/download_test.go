package release_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/release"
)

func TestDownloadWritesTheFileOrLeavesNothing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("0123456789"))
		case "/missing":
			http.NotFound(w, r)
		default:
			http.Error(w, "broken", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	cases := []struct {
		name    string
		path    string
		limit   int64
		wantErr string
		want    string
	}{
		{name: "a file", path: "/ok", limit: 10, want: "0123456789"},
		{name: "a file one byte over the limit", path: "/ok", limit: 9, wantErr: "larger than"},
		{name: "not found", path: "/missing", limit: 10, wantErr: "404"},
		{name: "the server fails", path: "/broken", limit: 10, wantErr: "500"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dest := filepath.Join(t.TempDir(), "file")

			err := release.Download(t.Context(), server.Client(), server.URL+tc.path, dest, tc.limit)
			if tc.wantErr == "" {
				if err != nil || string(readFile(t, dest)) != tc.want {
					t.Fatalf("Download = %v, content %q; want %q", err, readFile(t, dest), tc.want)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Download error = %v, want it to mention %q", err, tc.wantErr)
			}

			if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
				t.Fatalf("a failed download left a file behind: %v", statErr)
			}
		})
	}
}

func TestDownloadFailsWhenNobodyAnswers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	address := server.URL
	client := server.Client()
	server.Close()

	if err := release.Download(t.Context(), client, address+"/x", filepath.Join(t.TempDir(), "file"), 10); err == nil {
		t.Fatal("a download from a closed server succeeded")
	}
}

func TestDownloadDoesNotOverwriteAFile(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("new")) }))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest, []byte("old"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := release.Download(t.Context(), server.Client(), server.URL, dest, 10); err == nil || string(readFile(t, dest)) != "old" {
		t.Fatalf("Download error = %v, content %q; want the refusal and the old file", err, readFile(t, dest))
	}
}

func TestDownloadRefusesAnAddressItCannotRequest(t *testing.T) {
	t.Parallel()

	err := release.Download(t.Context(), nil, "http://[bad", filepath.Join(t.TempDir(), "file"), 10)
	if err == nil {
		t.Fatal("a malformed address was requested")
	}
}
