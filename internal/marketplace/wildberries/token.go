package wildberries

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/prankyman/wyo/internal/marketplace"
)

// fieldToken is the one field of the key form.
const fieldToken = "token"

// fieldPath is where a refusal of the token lies in the form (api/wyo.yaml, createAccount: `key.<field>`).
const fieldPath = "key." + fieldToken

// The refusal codes of a token read without calling anybody (api/wyo.yaml, createAccount).
const (
	codeRequired           = "required"
	codeKeyUnreadable      = "key_unreadable"
	codeKeyExpired         = "key_expired"
	codeKeyTypeNotAccepted = "key_type_not_accepted"
	codeCategoryMissing    = "category_missing"
)

// The token's types, the field acc (WBT §2.1, «Значение acc»), and their names in the connector's data.
var tokenTypes = map[int]string{1: "base", 2: "test", 3: "personal", 4: "service"}

// accepted are the types wyo takes: the Personal token, which Wildberries gives a program on the seller's own server, and
// the test token of its sandbox. «Базовый» is refused (its limits are too low for this work: the orders feed once in
// 3 hours, WBC §3), and so is «Сервисный» (it belongs to a service of Wildberries' catalogue, WBT §1.4).
var accepted = map[string]bool{"personal": true, "test": true}

// The bits of the field s (WBT §2.2: «Позиция бита отсчитывается от 0, где 0 — это младший бит»), by the connector's
// code of each category. Bit 30 is «Токен только на чтение».
var categoryBits = []struct {
	bit  uint
	code string
}{
	{1, "content"},
	{2, "analytics"},
	{3, "prices"},
	{4, categoryMarketplace},
	{5, categoryStatistics},
	{6, "promotion"},
	{7, "feedbacks"},
	{9, "chat"},
	{10, "supplies"},
	{11, "returns"},
	{12, "documents"},
	{13, "finance"},
	{16, "users"},
}

const readOnlyBit = 30

// claims are the token's own fields (WBT §2.1): exp — the moment it stops working (RFC 7519 NumericDate), sid — the
// seller's profile, acc — the type, s — the categories, t — a test token. «Остальные поля payload служебные».
type claims struct {
	Exp *int64 `json:"exp"`
	Sid string `json:"sid"`
	Acc int    `json:"acc"`
	S   uint64 `json:"s"`
	T   bool   `json:"t"`
}

func refused(code string, params map[string]any) *marketplace.Refusal {
	return &marketplace.Refusal{Code: code, Field: fieldPath, Params: params}
}

// ReadKey implements marketplace.KeyChecker: the token's own fields, read without calling anybody («Токен представляет
// собой JWT согласно RFC 7519 … вы можете декодировать его», WBT §2).
func (a *Adapter) ReadKey(fields marketplace.Key, now time.Time) (marketplace.KeyFacts, marketplace.Key, error) {
	token := strings.TrimSpace(fields[fieldToken])
	if token == "" {
		return marketplace.KeyFacts{}, nil, refused(codeRequired, nil)
	}

	c, ok := decode(token)
	if !ok {
		return marketplace.KeyFacts{}, nil, refused(codeKeyUnreadable, nil)
	}

	kind, known := tokenTypes[c.Acc]
	switch {
	case !known || c.Sid == "":
		return marketplace.KeyFacts{}, nil, refused(codeKeyUnreadable, nil)
	case !accepted[kind]:
		return marketplace.KeyFacts{}, nil, refused(codeKeyTypeNotAccepted, map[string]any{"type": kind})
	}

	facts := marketplace.KeyFacts{
		Type: kind, Test: kind == "test" || c.T, ProfileID: c.Sid, Categories: categoriesOf(c.S), ReadOnly: c.S&(1<<readOnlyBit) != 0,
	}

	if c.Exp != nil {
		facts.ExpiresAt = time.Unix(*c.Exp, 0).UTC()

		if !now.Before(facts.ExpiresAt) {
			return marketplace.KeyFacts{}, nil, refused(codeKeyExpired, nil)
		}
	}

	// A test token «автоматически открывает все категории, доступные в песочнице» (WBT §1.5); which bits its s holds is
	// not sourced (WBT §4, point 5), so its categories are proven by /ping on the sandbox, not by its bits.
	if facts.Test {
		return facts, marketplace.Key{fieldToken: token}, nil
	}

	if !slices.Contains(facts.Categories, categoryMarketplace) {
		return marketplace.KeyFacts{}, nil, refused(codeCategoryMissing, map[string]any{"category": categoryMarketplace})
	}

	if !slices.Contains(facts.Categories, categoryStatistics) {
		facts.MissingCategories = []string{categoryStatistics}
	}

	return facts, marketplace.Key{fieldToken: token}, nil
}

// decode reads the middle part of a JWT: three parts separated by dots, the middle one base64url-encoded JSON.
func decode(token string) (claims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims{}, false
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, false
	}

	return c, true
}

func categoriesOf(s uint64) []string {
	var list []string

	for _, c := range categoryBits {
		if s&(1<<c.bit) != 0 {
			list = append(list, c.code)
		}
	}

	return list
}
