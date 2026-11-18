package settings_test

import (
	"errors"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/settings"
	"github.com/prankyman/wyo/internal/store"
	"github.com/prankyman/wyo/internal/testkit/storekit"
)

var moscow = time.FixedZone("MSK", 3*60*60)

func noon() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, moscow) }

func newService(t *testing.T) (*settings.Service, *store.Store) {
	t.Helper()

	st := storekit.Open(t)

	return settings.NewService(st, clock.NewFixed(noon())), st
}

func TestTheRegistryHoldsTheTwoSettingsOfThisPlanInTwoGroups(t *testing.T) {
	t.Parallel()

	svc, _ := newService(t)

	groups, err := svc.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}

	if len(groups) != 2 || groups[0].ID != "updates" || groups[1].ID != "security" {
		t.Fatalf("groups %+v", groups)
	}

	auto := groups[0].Settings[0]
	if auto.Key != "updates.auto" || auto.Kind != settings.KindBoolean || auto.Value != true || auto.Default != true || auto.Min != nil || auto.Max != nil {
		t.Fatalf("updates.auto: %+v", auto)
	}

	idle := groups[1].Settings[0]
	if idle.Key != "security.idleMinutes" || idle.Kind != settings.KindInteger || idle.Value != 60 || idle.Default != 60 ||
		idle.Min == nil || *idle.Min != 5 || idle.Max == nil || *idle.Max != 720 {
		t.Fatalf("security.idleMinutes: %+v", idle)
	}
}

func TestEverySettingIsRegisteredOnceWithAValidDefaultInItsOwnGroup(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, def := range settings.Registry() {
		if seen[def.Key] {
			t.Fatalf("%s is registered twice", def.Key)
		}

		seen[def.Key] = true

		if len(def.Key) <= len(def.Group)+1 || def.Key[:len(def.Group)+1] != def.Group+"." {
			t.Fatalf("%s does not begin with its group %s", def.Key, def.Group)
		}

		if issue := def.Validate(def.Default); issue != nil {
			t.Fatalf("the default of %s is refused by its own limits: %+v", def.Key, issue)
		}
	}
}

func TestAGroupIsSavedAndReadBack(t *testing.T) {
	t.Parallel()

	svc, st := newService(t)
	ctx := t.Context()

	saved, err := svc.Save(ctx, "security", map[string]any{"security.idleMinutes": float64(90)}) // JSON numbers are float64
	if err != nil || saved.Settings[0].Value != 90 {
		t.Fatalf("Save: %+v, %v", saved, err)
	}

	if minutes, err := svc.IdleMinutes(ctx); err != nil || minutes != 90 {
		t.Fatalf("IdleMinutes: %d, %v", minutes, err)
	}

	row, err := st.Read().Setting(ctx, "security.idleMinutes")
	if err != nil || row.Value != "90" || !row.ChangedAt.Equal(noon()) {
		t.Fatalf("the row: %+v, %v", row, err)
	}

	if _, err := svc.Save(ctx, "updates", map[string]any{"updates.auto": false}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if auto, err := svc.AutoUpdates(ctx); err != nil || auto {
		t.Fatalf("AutoUpdates: %v, %v", auto, err)
	}
}

func TestAnInvalidSaveSavesNothingAndSaysWhyForEveryField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		group  string
		values map[string]any
		want   []issue.Issue
	}{
		{
			name: "below the lowest", group: "security", values: map[string]any{"security.idleMinutes": 4},
			want: []issue.Issue{{Path: "security.idleMinutes", Code: "out_of_range", Params: map[string]any{"min": 5, "max": 720}}},
		},
		{
			name: "above the highest", group: "security", values: map[string]any{"security.idleMinutes": 721},
			want: []issue.Issue{{Path: "security.idleMinutes", Code: "out_of_range", Params: map[string]any{"min": 5, "max": 720}}},
		},
		{
			name: "text for a number", group: "security", values: map[string]any{"security.idleMinutes": "60"},
			want: []issue.Issue{{Path: "security.idleMinutes", Code: "wrong_kind"}},
		},
		{
			name: "a fraction for a whole number", group: "security", values: map[string]any{"security.idleMinutes": 60.5},
			want: []issue.Issue{{Path: "security.idleMinutes", Code: "wrong_kind"}},
		},
		{
			name: "a switch for a number", group: "security", values: map[string]any{"security.idleMinutes": true},
			want: []issue.Issue{{Path: "security.idleMinutes", Code: "wrong_kind"}},
		},
		{
			name: "a number for a switch", group: "updates", values: map[string]any{"updates.auto": 1},
			want: []issue.Issue{{Path: "updates.auto", Code: "wrong_kind"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc, st := newService(t)

			_, err := svc.Save(t.Context(), tc.group, tc.values)

			var invalid *issue.Error
			if !errors.As(err, &invalid) || len(invalid.Issues) != len(tc.want) {
				t.Fatalf("got %v, want %+v", err, tc.want)
			}

			for i, want := range tc.want {
				got := invalid.Issues[i]
				if got.Path != want.Path || got.Code != want.Code || len(got.Params) != len(want.Params) || got.Params["min"] != want.Params["min"] || got.Params["max"] != want.Params["max"] {
					t.Fatalf("issue %d: got %+v, want %+v", i, got, want)
				}
			}

			if rows, _ := st.Read().Settings(t.Context()); len(rows) != 0 {
				t.Fatalf("a refused save wrote %+v", rows)
			}
		})
	}
}

func TestOneRefusedValueStopsTheWholeGroupFromBeingSaved(t *testing.T) {
	t.Parallel()

	svc, st := newService(t)

	// Two keys of one group would show it; this plan's groups hold one each, so the proof is a key of another group.
	_, err := svc.Save(t.Context(), "security", map[string]any{"security.idleMinutes": 90, "updates.auto": false})
	if !errors.Is(err, settings.ErrUnknownSetting) {
		t.Fatalf("got %v, want a refusal of a key that is not in the group", err)
	}

	if rows, _ := st.Read().Settings(t.Context()); len(rows) != 0 {
		t.Fatalf("the allowed key was saved although the other was refused: %+v", rows)
	}
}

func TestAnUnknownGroupIsRefused(t *testing.T) {
	t.Parallel()

	svc, _ := newService(t)

	if _, err := svc.Save(t.Context(), "nothing", nil); !errors.Is(err, settings.ErrUnknownGroup) {
		t.Fatalf("Save: %v", err)
	}

	if _, err := svc.Group(t.Context(), "nothing"); !errors.Is(err, settings.ErrUnknownGroup) {
		t.Fatalf("Group: %v", err)
	}
}

func TestADamagedStoredValueFallsBackToTheDefault(t *testing.T) {
	t.Parallel()

	svc, st := newService(t)
	ctx := t.Context()

	must(t, st.Do(ctx, func(q *store.Queries) error {
		if err := q.PutSetting(ctx, store.Setting{Key: "security.idleMinutes", Value: "soon", ChangedAt: noon()}); err != nil {
			return err
		}

		return q.PutSetting(ctx, store.Setting{Key: "updates.auto", Value: "maybe", ChangedAt: noon()})
	}))

	if minutes, err := svc.IdleMinutes(ctx); err != nil || minutes != 60 {
		t.Fatalf("IdleMinutes: %d, %v", minutes, err)
	}

	if auto, err := svc.AutoUpdates(ctx); err != nil || !auto {
		t.Fatalf("AutoUpdates: %v, %v", auto, err)
	}
}

func TestTheUpdatesChoiceOfInitIsWrittenOnlyIntoANewDatabase(t *testing.T) {
	t.Parallel()

	svc, st := newService(t)

	if !st.Created() {
		t.Fatal("the test needs a new database")
	}

	must(t, svc.SeedUpdates(t.Context(), false))

	if auto, _ := svc.AutoUpdates(t.Context()); auto {
		t.Fatal("wyo init --updates=manual must switch automatic updates off in a new database")
	}

	// The same database opened again is not new: init changes nothing that exists.
	path := st.Path()
	must(t, st.Close())

	again := storekit.OpenAt(t, path)
	if again.Created() {
		t.Fatal("an existing database is not created again")
	}

	reopened := settings.NewService(again, clock.NewFixed(noon()))
	must(t, reopened.SeedUpdates(t.Context(), true))

	if auto, _ := reopened.AutoUpdates(t.Context()); auto {
		t.Fatal("wyo init on an existing database must change nothing")
	}
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}
