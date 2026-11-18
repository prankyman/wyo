package storekit_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/testkit/storekit"
)

func TestOpenGivesAMigratedDatabaseOfItsOwn(t *testing.T) {
	t.Parallel()

	first, second := storekit.Open(t), storekit.Open(t)

	if first.Path() == second.Path() {
		t.Fatal("two tests must never share a database file")
	}

	if err := first.Ready(t.Context()); err != nil {
		t.Fatalf("the database is not ready: %v", err)
	}
}

func TestOpenAtMakesTheFileWhereItIsTold(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "wyo.db")

	if storekit.OpenAt(t, path).Created() != true {
		t.Fatal("a new file is created")
	}
}

// recorder stands for the test of a helper that fails: it remembers what the helper reported instead of failing the test
// that checks it. Fatalf ends only the goroutine it is called in, as the real one does.
type recorder struct {
	testing.TB
	failures []string
}

func (*recorder) Helper() {}

func (*recorder) Context() context.Context { return context.Background() }

func (*recorder) Cleanup(func()) {}

func (r *recorder) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))

	runtime.Goexit()
}

func TestOpenAtFailsTheTestThatCannotHaveItsDatabase(t *testing.T) {
	t.Parallel()

	r := &recorder{}
	missing := filepath.Join(t.TempDir(), "no", "such", "folder", "wyo.db")
	done := make(chan struct{})

	go func() {
		defer close(done)

		storekit.OpenAt(r, missing)
	}()

	<-done

	if len(r.failures) != 1 || !strings.Contains(r.failures[0], missing) {
		t.Fatalf("failures %v, want one naming %s", r.failures, missing)
	}
}
