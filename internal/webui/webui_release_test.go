//go:build !wyotest

package webui_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/webui"
)

func TestReleaseBuildHasNoShowcase(t *testing.T) {
	t.Parallel()

	opts := webui.DefaultOptions()
	if opts.Showcase {
		t.Fatal("a build without the wyotest tag must not carry the showcase page")
	}

	h := webui.New(builtInterface(), opts)

	for _, target := range []string{"/showcase", "/ru/showcase"} {
		if got := ask(t, h, http.MethodGet, target); got.Status != http.StatusNotFound {
			t.Fatalf("%s answered %d in a release build, want 404", target, got.Status)
		}
	}
}
