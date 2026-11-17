//go:build wyotest

package webui_test

import (
	"net/http"
	"testing"

	"github.com/prankyman/wyo/internal/webui"
)

func TestTaggedBuildHasTheShowcase(t *testing.T) {
	t.Parallel()

	opts := webui.DefaultOptions()
	if !opts.Showcase {
		t.Fatal("a build with the wyotest tag must carry the showcase page")
	}

	h := webui.New(builtInterface(), opts)

	for _, target := range []string{"/showcase", "/ru/showcase"} {
		if got := ask(t, h, http.MethodGet, target); got.Status != http.StatusOK {
			t.Fatalf("%s answered %d in a tagged build, want 200", target, got.Status)
		}
	}
}
