package wildberries

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// TokenType is the field acc of a token: «Значение acc» of the section «Как устроен токен» of api/wildberries/01-general.yaml
// (.agents/research/wildberries-token-check-2026-11-18.md, section 2.1).
type TokenType int

// The four types.
const (
	TokenBase     TokenType = 1
	TokenTest     TokenType = 2
	TokenPersonal TokenType = 3
	TokenService  TokenType = 4
)

// The bits of the field s that the connector reads (the same section, «Значения бит»; bit 0 is the lowest).
const (
	CategoryMarketplace uint64 = 1 << 4
	CategoryStatistics  uint64 = 1 << 5
	readOnlyBit         uint64 = 1 << 30
)

// TokenClaims are the fields of a made-up token.
type TokenClaims struct {
	ID         string
	Seller     string
	Type       TokenType
	Expires    time.Time
	Categories uint64
	ReadOnly   bool
}

// MakeToken writes a made-up token in the shape of Wildberries' own: a JWT (RFC 7519) whose middle part holds id, sid,
// acc, s, t, exp and for. The signature is made up too: the fake never checks it, and neither does the connector, which
// only reads the token's fields.
func MakeToken(c TokenClaims) string {
	s := c.Categories
	if c.ReadOnly {
		s |= readOnlyBit
	}

	claims := map[string]any{"id": c.ID, "sid": c.Seller, "acc": int(c.Type), "s": s, "t": c.Type == TokenTest}

	switch c.Type {
	case TokenPersonal:
		claims["for"] = "self"
	case TokenService:
		claims["for"] = "asid:fake-service"
	case TokenBase, TokenTest:
	}

	if !c.Expires.IsZero() {
		claims["exp"] = c.Expires.Unix()
	}

	header, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT"}) // a map of strings always marshals
	payload, _ := json.Marshal(claims)                                         // so do these plain values

	encode := base64.RawURLEncoding.EncodeToString

	return encode(header) + "." + encode(payload) + "." + encode([]byte("fake-signature"))
}
