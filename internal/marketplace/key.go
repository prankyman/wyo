package marketplace

import (
	"encoding/json"
	"strings"
)

// Key is a key's fields by the names of its adapter's key form (KeyField.Name), as the owner typed them.
type Key map[string]string

// ShownField is a field of a key that is not secret, with its value: the screens show it in full (a number that tells
// two accounts of one marketplace apart, such as a client's number).
type ShownField struct {
	Name  string
	Value string
}

// packedPrefix starts a packed key; a text without it was stored before keys were packed.
const packedPrefix = "wyo.key.v1:"

// PackKey is THE one way a key's fields become the one text the secret store keeps. A key of one field is its value, as
// every release before keys of several fields kept it; a key of several fields is the fields of the form, as JSON, after
// a prefix. A field the form does not have is left out.
func PackKey(fields []KeyField, key Key) string {
	if len(fields) == 1 {
		return key[fields[0].Name]
	}

	kept := make(map[string]string, len(fields))
	for _, f := range fields {
		kept[f.Name] = key[f.Name]
	}

	raw, _ := json.Marshal(kept) // a map of texts always encodes

	return packedPrefix + string(raw)
}

// UnpackKey reads a text of the secret store back into the key's fields. A text that is not packed — a key of one field,
// and every key stored before keys of several fields existed — is the first field's value, so an account added then goes
// on working without its key being touched.
func UnpackKey(fields []KeyField, text string) Key {
	if rest, packed := strings.CutPrefix(text, packedPrefix); packed && len(fields) != 1 {
		var key Key
		if err := json.Unmarshal([]byte(rest), &key); err == nil {
			return key
		}
	}

	first := ""
	if len(fields) > 0 {
		first = fields[0].Name
	}

	return Key{first: text}
}

// LastFour is the last four characters of the key's first secret field, the only part of a key ever shown; "" when the
// form has no secret field.
func LastFour(fields []KeyField, key Key) string {
	for _, f := range fields {
		if !f.Secret {
			continue
		}

		runes := []rune(key[f.Name])
		if len(runes) <= 4 {
			return string(runes)
		}

		return string(runes[len(runes)-4:])
	}

	return ""
}

// Secrets lists the values of the key's secret fields: what must never reach a log line, a journal row or an answer.
// Every value of a key whose form has no fields is secret.
func (k Key) Secrets(fields []KeyField) []string {
	if len(fields) == 0 {
		list := make([]string, 0, len(k))
		for _, v := range k {
			list = append(list, v)
		}

		return list
	}

	var list []string

	for _, f := range fields {
		if f.Secret && k[f.Name] != "" {
			list = append(list, k[f.Name])
		}
	}

	return list
}

// Shown lists the key's fields that are not secret, in the form's order.
func (k Key) Shown(fields []KeyField) []ShownField {
	var list []ShownField

	for _, f := range fields {
		if !f.Secret {
			list = append(list, ShownField{Name: f.Name, Value: k[f.Name]})
		}
	}

	return list
}
