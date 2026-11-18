// Package messages carries the interface's text files inside the wyo server, so the commands of the wyo server speak the
// same words as its pages and a second language file covers both (constitution, principle I).
package messages

import "embed"

// Files holds every language file of this folder, e.g. ru.json.
//
//go:embed *.json
var Files embed.FS
