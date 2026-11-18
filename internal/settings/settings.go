// Package settings is the registry of the wyo server's settings: one list, with each setting's kind, limits, default
// and group. A later plan adds a setting by adding one line here; the API, the page «Настройки» and the checks of a
// saved value all read this list. Texts are never here: the interface words them by key
// (`settings.<key>.label`, `settings.<key>.help`, `settings.group.<id>.title`).
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/issue"
	"github.com/prankyman/wyo/internal/store"
)

// Kind is what a setting holds.
type Kind string

// The kinds.
const (
	KindBoolean Kind = "boolean"
	KindInteger Kind = "integer"
)

// The keys other packages read.
const (
	KeyUpdatesAuto = "updates.auto"
	KeyIdleMinutes = "security.idleMinutes"
)

var (
	// ErrUnknownGroup is a group the registry does not have.
	ErrUnknownGroup = errors.New("unknown settings group")
	// ErrUnknownSetting is a key that is not a setting of the group it was sent to.
	ErrUnknownSetting = errors.New("unknown setting")
)

// Definition is one setting of the registry.
type Definition struct {
	Key   string
	Group string
	Kind  Kind
	// Default is a bool for KindBoolean and an int for KindInteger.
	Default any
	// Min and Max are the limits of an integer, both included; nil for a switch.
	Min *int
	Max *int
}

func limit(n int) *int { return &n }

// registry is the list. Order matters: it is the order the page draws groups and settings in.
var registry = []Definition{
	{Key: KeyUpdatesAuto, Group: "updates", Kind: KindBoolean, Default: true},
	{Key: KeyIdleMinutes, Group: "security", Kind: KindInteger, Default: 60, Min: limit(5), Max: limit(720)},
}

// Registry returns the list of settings.
func Registry() []Definition {
	return append([]Definition(nil), registry...)
}

// Validate says why a value is refused, or nil when the setting accepts it.
func (d Definition) Validate(value any) *issue.Issue {
	_, bad := d.Encode(value)

	return bad
}

// Encode turns a value into the text kept in the database, or says why it is refused. A whole number sent as JSON
// arrives as a float64; a fraction is a value of the wrong kind.
func (d Definition) Encode(value any) (string, *issue.Issue) {
	wrongKind := &issue.Issue{Path: d.Key, Code: issue.WrongKind}

	switch d.Kind {
	case KindBoolean:
		b, ok := value.(bool)
		if !ok {
			return "", wrongKind
		}

		return strconv.FormatBool(b), nil
	case KindInteger:
		n, ok := wholeNumber(value)
		if !ok {
			return "", wrongKind
		}

		if (d.Min != nil && n < *d.Min) || (d.Max != nil && n > *d.Max) {
			return "", &issue.Issue{Path: d.Key, Code: issue.OutOfRange, Params: d.limits()}
		}

		return strconv.Itoa(n), nil
	}

	return "", wrongKind
}

// limits are the limits an integer has, as the parameters of the refusal's text.
func (d Definition) limits() map[string]any {
	params := map[string]any{}

	if d.Min != nil {
		params["min"] = *d.Min
	}

	if d.Max != nil {
		params["max"] = *d.Max
	}

	return params
}

func wholeNumber(value any) (int, bool) {
	switch n := value.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != math.Trunc(n) || math.Abs(n) > math.MaxInt32 {
			return 0, false
		}

		return int(n), true
	case json.Number:
		v, err := strconv.Atoi(n.String())

		return v, err == nil
	}

	return 0, false
}

// decode turns a stored text back into a value; ok is false for a text the setting could not have written.
func (d Definition) decode(text string) (any, bool) {
	switch d.Kind {
	case KindBoolean:
		b, err := strconv.ParseBool(text)

		return b, err == nil
	case KindInteger:
		n, err := strconv.Atoi(text)
		if err != nil || (d.Min != nil && n < *d.Min) || (d.Max != nil && n > *d.Max) {
			return nil, false
		}

		return n, true
	}

	return nil, false
}

// Setting is a setting with its current value, as the API shows it.
type Setting struct {
	Key     string
	Kind    Kind
	Value   any
	Default any
	Min     *int
	Max     *int
}

// Group is the settings of one group.
type Group struct {
	ID       string
	Settings []Setting
}

// Service reads and saves the settings of one database.
type Service struct {
	store *store.Store
	clock clock.Clock
}

// NewService returns the settings of st; saved values carry the time from clk.
func NewService(st *store.Store, clk clock.Clock) *Service {
	return &Service{store: st, clock: clk}
}

// Groups lists every group with the current value of each setting.
func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	saved, err := s.saved(ctx)
	if err != nil {
		return nil, err
	}

	var groups []Group

	for _, def := range registry {
		if len(groups) == 0 || groups[len(groups)-1].ID != def.Group {
			groups = append(groups, Group{ID: def.Group})
		}

		last := &groups[len(groups)-1]
		last.Settings = append(last.Settings, view(def, saved))
	}

	return groups, nil
}

// Group lists one group; ErrUnknownGroup when the registry has none of that id.
func (s *Service) Group(ctx context.Context, id string) (Group, error) {
	groups, err := s.Groups(ctx)
	if err != nil {
		return Group{}, err
	}

	for _, g := range groups {
		if g.ID == id {
			return g, nil
		}
	}

	return Group{}, ErrUnknownGroup
}

// Save saves values of one group, all or none: when any value is refused, nothing is saved and the issue.Error names
// every refused value. A key that is not a setting of the group is ErrUnknownSetting.
func (s *Service) Save(ctx context.Context, id string, values map[string]any) (Group, error) {
	defs := map[string]Definition{}
	known := false

	for _, def := range registry {
		if def.Group == id {
			defs[def.Key] = def
			known = true
		}
	}

	if !known {
		return Group{}, ErrUnknownGroup
	}

	rows := make([]store.Setting, 0, len(values))

	var issues []issue.Issue

	for _, def := range registry { // in the registry's order, so the answer does not depend on the map
		value, sent := values[def.Key]
		if !sent || def.Group != id {
			continue
		}

		text, bad := def.Encode(value)
		if bad != nil {
			issues = append(issues, *bad)

			continue
		}

		rows = append(rows, store.Setting{Key: def.Key, Value: text, ChangedAt: s.clock.Now()})
	}

	for key := range values {
		if _, ok := defs[key]; !ok {
			return Group{}, fmt.Errorf("%w: %q in the group %q", ErrUnknownSetting, key, id)
		}
	}

	if len(issues) > 0 {
		return Group{}, &issue.Error{Issues: issues}
	}

	err := s.store.Do(ctx, func(q *store.Queries) error {
		for _, row := range rows {
			if err := q.PutSetting(ctx, row); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return Group{}, fmt.Errorf("save the settings: %w", err)
	}

	return s.Group(ctx, id)
}

// IdleMinutes is how long a session may stay without its owner's own action.
func (s *Service) IdleMinutes(ctx context.Context) (int, error) {
	v, err := s.value(ctx, KeyIdleMinutes)
	if err != nil {
		return 0, err
	}

	minutes, _ := v.(int)

	return minutes, nil
}

// AutoUpdates says whether the updater installs a newer version by itself.
func (s *Service) AutoUpdates(ctx context.Context) (bool, error) {
	v, err := s.value(ctx, KeyUpdatesAuto)
	if err != nil {
		return false, err
	}

	auto, _ := v.(bool)

	return auto, nil
}

// SeedUpdates writes the choice of `wyo init --updates=auto|manual` — and only into a database this start made. A
// database that already existed keeps what it has: init changes nothing that exists.
func (s *Service) SeedUpdates(ctx context.Context, auto bool) error {
	if !s.store.Created() {
		return nil
	}

	row := store.Setting{Key: KeyUpdatesAuto, Value: strconv.FormatBool(auto), ChangedAt: s.clock.Now()}

	if err := s.store.Do(ctx, func(q *store.Queries) error { return q.PutSettingIfAbsent(ctx, row) }); err != nil {
		return fmt.Errorf("seed the updates setting: %w", err)
	}

	return nil
}

func (s *Service) value(ctx context.Context, key string) (any, error) {
	saved, err := s.saved(ctx)
	if err != nil {
		return nil, err
	}

	for _, def := range registry {
		if def.Key == key {
			return view(def, saved).Value, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrUnknownSetting, key)
}

func (s *Service) saved(ctx context.Context) (map[string]string, error) {
	rows, err := s.store.Read().Settings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the settings: %w", err)
	}

	saved := make(map[string]string, len(rows))
	for _, r := range rows {
		saved[r.Key] = r.Value
	}

	return saved, nil
}

// view is a setting with its value: the saved one, or the default when none was saved or the text is not one the
// setting could have written (the program wrote it, so this only guards against a hand-edited database).
func view(def Definition, saved map[string]string) Setting {
	value := def.Default

	if text, ok := saved[def.Key]; ok {
		if v, valid := def.decode(text); valid {
			value = v
		}
	}

	return Setting{Key: def.Key, Kind: def.Kind, Value: value, Default: def.Default, Min: def.Min, Max: def.Max}
}
