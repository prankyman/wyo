package auth_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/auth"
	"github.com/prankyman/wyo/internal/clock"
)

func TestTheLimiterBlocksAfterFiveFailuresAndLetsGoWhenTheOldestAgesOut(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(noon())
	limiter := auth.NewLimiter(clk, 5, 15*time.Minute)

	for i := range 5 {
		if wait, blocked := limiter.Blocked("login:anna"); blocked {
			t.Fatalf("blocked after %d failures (waits %v)", i, wait)
		}

		limiter.Fail("login:anna")
		clk.Advance(time.Minute) // the failures are at 12:00, 12:01 ... 12:04; now it is 12:05
	}

	wait, blocked := limiter.Blocked("login:anna")
	if !blocked || wait != 10*time.Minute {
		t.Fatalf("blocked %v, wait %v: the first failure (12:00) ages out at 12:15, ten minutes from 12:05", blocked, wait)
	}

	clk.Advance(10*time.Minute - time.Second)

	if _, blocked := limiter.Blocked("login:anna"); !blocked {
		t.Fatal("one second before the oldest failure ages out the key is still blocked")
	}

	clk.Advance(time.Second)

	if _, blocked := limiter.Blocked("login:anna"); blocked {
		t.Fatal("at the moment the oldest failure ages out the key is free")
	}
}

func TestTheLimiterCountsPerKeyAndTakesTheLongestWait(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(noon())
	limiter := auth.NewLimiter(clk, 2, 15*time.Minute)

	limiter.Fail("login:a") // 12:00
	clk.Advance(5 * time.Minute)
	limiter.Fail("login:a", "addr:1") // 12:05
	limiter.Fail("addr:1")            // 12:05

	if _, blocked := limiter.Blocked("login:b"); blocked {
		t.Fatal("an unrelated key is free")
	}

	wait, blocked := limiter.Blocked("login:a", "addr:1")
	if !blocked || wait != 15*time.Minute {
		t.Fatalf("blocked %v, wait %v: login:a is free at 12:15 (ten minutes), addr:1 at 12:20 (fifteen): the longest counts", blocked, wait)
	}
}

func TestClearForgetsTheFailuresOfTheKeysGiven(t *testing.T) {
	t.Parallel()

	limiter := auth.NewLimiter(clock.NewFixed(noon()), 1, time.Hour)

	limiter.Fail("login:a", "addr:1")
	limiter.Clear("login:a")

	if _, blocked := limiter.Blocked("login:a"); blocked {
		t.Fatal("a cleared key is free")
	}

	if _, blocked := limiter.Blocked("addr:1"); !blocked {
		t.Fatal("a key that was not cleared stays blocked")
	}
}

func TestTheLimiterDoesNotGrowWithoutEnd(t *testing.T) {
	t.Parallel()

	clk := clock.NewFixed(noon())
	limiter := auth.NewLimiter(clk, 5, time.Minute)

	for i := range 3000 {
		limiter.Fail(fmt.Sprintf("login:user%d", i))
	}

	clk.Advance(2 * time.Minute)
	limiter.Fail("login:fresh") // a failure after the window forgets every key whose failures all aged out

	if got := limiter.Size(); got != 1 {
		t.Fatalf("the limiter still remembers %d keys", got)
	}
}
