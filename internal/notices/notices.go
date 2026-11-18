// Package notices is the bell's one model: a notice tells the owner that something needs attention. Every package that
// can need the owner's attention implements Source; the wyo server asks every source on each read of the bell, so a
// notice lasts exactly as long as its cause. A later plan adds a source, never another bell.
package notices

import (
	"context"
	"sort"
	"time"
)

// The sources. A notice's text is `notices.<code>` in messages/ru.json.
const (
	SourceUpdates     = "updates"
	SourceCertificate = "certificate"
	SourceBackups     = "backups"
	SourceAccounts    = "accounts"
	// SourceExternalAPI is the exchange with the fulfilment centre of «Внешний API» (Plan 4).
	SourceExternalAPI = "external_api"
)

// The tones, the same names as the interface's status tones.
const (
	ToneOK   = "ok"
	ToneWait = "wait"
	ToneWarn = "warn"
	ToneStop = "stop"
	ToneInfo = "info"
)

// Notice is one line behind the bell.
type Notice struct {
	// ID stays the same while the cause lasts, so the interface can tell a new notice from one already seen.
	ID     string
	Source string
	Code   string
	Tone   string
	Params map[string]any
	At     *time.Time
}

// Source is anything that can need the owner's attention.
type Source interface {
	Notices(ctx context.Context) ([]Notice, error)
}

// severity orders the bell: what stops work first, information last.
var severity = map[string]int{ToneStop: 0, ToneWarn: 1, ToneWait: 2, ToneInfo: 3, ToneOK: 4}

// Collect asks every source and returns their notices, the most severe first. A source that cannot answer is left out:
// the bell must still show what the other sources know.
func Collect(ctx context.Context, sources []Source) []Notice {
	all := []Notice{}

	for _, s := range sources {
		list, err := s.Notices(ctx)
		if err != nil {
			continue
		}

		all = append(all, list...)
	}

	sort.SliceStable(all, func(i, j int) bool { return severity[all[i].Tone] < severity[all[j].Tone] })

	return all
}
