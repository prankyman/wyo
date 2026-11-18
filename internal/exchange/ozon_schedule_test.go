package exchange_test

import (
	"testing"
	"time"
)

func TestWhatOzonIsStillMakingIsAskedAgainOnTheScheduleOfEachStep(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// started is the write that begins the step, and polled is what the wyo server asks while Ozon is slow.
		started, polled string
		// slow makes Ozon slow at the step for good.
		slow func(t *testing.T, r *ozRig, number string)
		// fast is how often it is asked during the first fastFor after the step began, and after is how often after.
		fast, fastFor, after time.Duration
	}{
		{
			name: "the verdict on the codes", started: "/v6/fbs/posting/product/exemplar/set", polled: "/v5/fbs/posting/product/exemplar/status",
			fast: 20 * time.Second, fastFor: 10 * time.Minute, after: 5 * time.Minute,
			slow: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/postings/exemplar-check", map[string]any{"posting_number": number, "in_process_readings": 100000})
			},
		},
		{
			name: "the packing the report started", started: "/v4/posting/fbs/ship", polled: "/v3/posting/fbs/get",
			fast: 20 * time.Second, fastFor: 10 * time.Minute, after: 5 * time.Minute,
			slow: func(t *testing.T, r *ozRig, number string) {
				t.Helper()
				r.ozControl(t, "/__control/postings/ship", map[string]any{"posting_number": number, "readings": 100000})
			},
		},
		{
			name: "the label's task", started: "/v3/posting/fbs/package-label/create", polled: "/v2/posting/fbs/package-label/get",
			fast: 10 * time.Second, fastFor: 5 * time.Minute, after: 5 * time.Minute,
			slow: func(t *testing.T, r *ozRig, _ string) {
				t.Helper()
				r.ozControl(t, "/__control/labels", map[string]any{"readings": 100000})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newOzonRig(t)
			r.connect(t)

			number := r.walkPosting(t, 1)
			r.reserved(t, number)
			tc.slow(t, r, number)
			r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

			// The jobs run every 10 seconds for an hour; the states job, which reads the same postings, is left out.
			for range 360 {
				r.kit.Clock.Advance(10 * time.Second)

				_ = r.svc.RunEvents(t.Context())
				_ = r.svc.RunVerdicts(t.Context())
			}

			began := r.ozCallsTo(t, tc.started)
			if len(began) != 1 {
				t.Fatalf("%s was written %d times", tc.started, len(began))
			}

			var polls []time.Time

			for _, c := range r.ozCallsTo(t, tc.polled) {
				if c.At.After(began[0].At) {
					polls = append(polls, c.At)
				}
			}

			if len(polls) < 20 {
				t.Fatalf("%s was asked only %d times in an hour", tc.polled, len(polls))
			}

			// Each time it is asked again after the period that its age says: the fast one while the step is young, the slow one after.
			for i := 1; i < len(polls); i++ {
				want := tc.after
				if polls[i-1].Sub(began[0].At) < tc.fastFor {
					want = tc.fast
				}

				if got := polls[i].Sub(polls[i-1]); got != want {
					t.Fatalf("%s was asked again %v after the reading made %v into the step, want %v", tc.polled, got, polls[i-1].Sub(began[0].At), want)
				}
			}
		})
	}
}

func TestTheLabelIsAskedForSixtySecondsAfterThePostingIsReadAsPacked(t *testing.T) {
	t.Parallel()

	r := newOzonRig(t)
	r.connect(t)

	number := r.walkPosting(t, 1)
	r.reserved(t, number)
	r.packWith(t, number, shirtsAndSocks([2]int{1, 1}, 1)...)

	for range 40 {
		r.verdicts(t, 10*time.Second)
	}

	tasks := r.ozCallsTo(t, "/v3/posting/fbs/package-label/create")
	if len(tasks) != 1 {
		t.Fatalf("%d label tasks were made", len(tasks))
	}

	// The reading that found the posting packed is the last of the posting before the task.
	var found time.Time

	for _, c := range r.ozCallsTo(t, "/v3/posting/fbs/get") {
		if c.At.Before(tasks[0].At) {
			found = c.At
		}
	}

	if got := tasks[0].At.Sub(found); got != 60*time.Second {
		t.Fatalf("the label was asked for %v after the posting was read as packed, want 60s", got)
	}
}
