package ozon_test

import (
	"net/http"
	"testing"
	"time"
)

func TestAKeyThatIsSetUpWithoutRolesIsAnAdminForThreeMonths(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.control("setup", map[string]any{"client_id": r.clientID}, nil)

	var out struct {
		ExpiresAt time.Time `json:"expires_at"`
		Roles     []struct {
			Name string `json:"name"`
		} `json:"roles"`
	}

	r.answer(pathRoles, nil, &out)

	if len(out.Roles) != 1 || out.Roles[0].Name != "Admin" || !out.ExpiresAt.Equal(midday.AddDate(0, 3, 0)) {
		t.Fatalf("the default key is %+v", out)
	}
}

func TestAKeyThatWasNeverSetUpIsRefusedAsInvalid(t *testing.T) {
	t.Parallel()

	r := newRig(t)

	for _, path := range allMethods {
		refused(t, r.call(path, nil), http.StatusUnauthorized, wordsInvalidKey)
	}
}
