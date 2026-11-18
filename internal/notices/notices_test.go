package notices_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/notices"
)

type fixedSource struct {
	list []notices.Notice
	err  error
}

func (s fixedSource) Notices(context.Context) ([]notices.Notice, error) { return s.list, s.err }

func TestCollectJoinsEverySourceAndOrdersBySeverity(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	got := notices.Collect(context.Background(), []notices.Source{
		fixedSource{list: []notices.Notice{{ID: "backups:old", Source: notices.SourceBackups, Code: "backup_old", Tone: notices.ToneWarn}}},
		fixedSource{err: errors.New("cannot read")},
		fixedSource{list: []notices.Notice{
			{ID: "updates:waiting", Source: notices.SourceUpdates, Code: "update_waiting", Tone: notices.ToneInfo, At: &at},
			{ID: "certificate:failed", Source: notices.SourceCertificate, Code: "certificate_failed", Tone: notices.ToneStop},
		}},
	})

	want := []string{"certificate:failed", "backups:old", "updates:waiting"}
	if len(got) != len(want) {
		t.Fatalf("got %d notices %+v, want %v", len(got), got, want)
	}

	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("notice %d is %q, want %q (all: %+v)", i, got[i].ID, id, got)
		}
	}
}

func TestCollectOfNoSourcesIsAnEmptyListNotNil(t *testing.T) {
	t.Parallel()

	got := notices.Collect(context.Background(), nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty list", got)
	}
}
