package exchange_test

import (
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/testkit/synckit"
)

// While «Внешний API» is empty the exchange's jobs have nothing to do, so they wake once every 10 minutes instead of every 20
// seconds or every minute; filled, each job keeps its own period; emptied again, they slow down again.
func TestTheExchangesJobsWakeRarelyWhileTheSettingIsEmpty(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	at := synckit.Noon()

	wakes := func() map[string]time.Duration {
		out := map[string]time.Duration{}
		for _, job := range r.svc.Jobs() {
			out[job.Name] = job.Plan.Next(at, synckit.Middle{}).Sub(at)
		}

		return out
	}

	idle := wakes()
	for _, name := range []string{"exchange.orders", "exchange.states", "exchange.events", "exchange.verdicts"} {
		if idle[name] < 10*time.Minute {
			t.Fatalf("with the setting empty %s wakes after %v, want at least 10 minutes", name, idle[name])
		}
	}

	r.connect(t)

	filled := wakes()
	if filled["exchange.verdicts"] > 30*time.Second || filled["exchange.orders"] > 2*time.Minute || filled["exchange.events"] > 2*time.Minute {
		t.Fatalf("with the setting filled the jobs keep their periods: %v", filled)
	}

	if err := r.svc.Disconnect(t.Context()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if again := wakes(); again["exchange.verdicts"] < 10*time.Minute {
		t.Fatalf("after «Отключить» the verdicts wake after %v, want at least 10 minutes", again["exchange.verdicts"])
	}
}
