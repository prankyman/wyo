package httpapi

import (
	"net/http"

	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/settings"
)

func groupBody(g settings.Group) gen.SettingsGroup {
	list := make([]gen.Setting, 0, len(g.Settings))

	for _, s := range g.Settings {
		list = append(list, gen.Setting{
			Key: s.Key, Kind: gen.SettingKind(s.Kind), Value: s.Value, DefaultValue: s.Default, Min: s.Min, Max: s.Max,
		})
	}

	return gen.SettingsGroup{Id: g.ID, Settings: list}
}

// GetSettings answers GET /api/v1/settings.
func (a *api) GetSettings(w http.ResponseWriter, r *http.Request) {
	groups, err := a.deps.Settings.Groups(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.SettingsList{Groups: make([]gen.SettingsGroup, 0, len(groups))}
	for _, g := range groups {
		body.Groups = append(body.Groups, groupBody(g))
	}

	writeJSON(w, http.StatusOK, body)
}

// SaveSettingsGroup answers PUT /api/v1/settings/{group}.
func (a *api) SaveSettingsGroup(w http.ResponseWriter, r *http.Request, group string) {
	var in gen.SettingsValues
	if !decode(w, r, &in) {
		return
	}

	if in.Values == nil {
		BadRequest(w, r, nil)

		return
	}

	saved, err := a.deps.Settings.Save(r.Context(), group, in.Values)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, groupBody(saved))
}
