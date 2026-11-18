package ozon

import "strings"

// The roles of a key (OZC sections 2 and 3). Ozon's table of roles is from 2024 and names none of the methods this fake
// answers, so the fake follows the rule the plan decided on: a key may call a method when it has the role «Admin», or a role
// whose list holds the method's exact path, or holds a beginning of the path that ends at a «/» (Ozon's example of
// /v1/roles lists the role «Posting FBS» with «/v1/posting»). /v1/roles itself is open to every key: it is how the key is read.
const (
	roleAdmin = "Admin"
	rolesPath = "/v1/roles"
)

// The states of a key that make Ozon refuse a call, set by /__control/setup. An empty state is a key that works.
const (
	keyDeactivated    = "deactivated"
	keyIPRestricted   = "ip_restricted"
	keyOfferNotSigned = "offer_not_signed"
)

var keyStates = []string{"", keyDeactivated, keyIPRestricted, keyOfferNotSigned}

// refusalFor says whether the key may not call the method, and why. The roles method is never refused here, so that a key
// that has ended can still be read for its end date (OZC section 2: «Получите дату истечения срока действия ключа в
// параметре expires_at метода /v1/roles»).
func (st *state) refusalFor(path string) (refusal, bool) {
	if path == rolesPath {
		return refusal{}, false
	}

	switch st.keyState {
	case keyDeactivated:
		return refusalDeactivated, true
	case keyIPRestricted:
		return refusalIPRestricted, true
	case keyOfferNotSigned:
		return refusalOfferNotSigned, true
	}

	// An ended key (OZC section 2: «Срок действия ключа — 3 месяца»): Ozon's words for it are not documented, and the fake's
	// choice, not Ozon's, is the words of a key that was deactivated.
	if !st.expiresAt.After(st.now()) {
		return refusalDeactivated, true
	}

	if !st.mayCall(path) {
		return refusalMissingRole, true
	}

	return refusal{}, false
}

func (st *state) mayCall(path string) bool {
	for _, r := range st.roles {
		if r.Name == roleAdmin {
			return true
		}

		for _, listed := range r.Methods {
			if opens(listed, path) {
				return true
			}
		}
	}

	return false
}

// opens says whether a listed method opens a path: it is the path, or a beginning of it that ends at a «/» (a listed
// /v3/posting opens /v3/posting/fbs/get, a listed /v3/post does not).
func opens(listed, path string) bool {
	if listed == "" {
		return false
	}

	rest, found := strings.CutPrefix(path, listed)

	return found && (rest == "" || rest[0] == '/' || strings.HasSuffix(listed, "/"))
}
