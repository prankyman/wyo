package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fakes/fulfilment"
	"github.com/prankyman/wyo/internal/fakes/ozon"
	"github.com/prankyman/wyo/internal/fakes/wildberries"
	"github.com/prankyman/wyo/internal/fakes/yandex"
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/server"
)

const (
	exitOK       = 0
	exitFailure  = 1
	exitBadInput = 2

	defaultMarketplaces = "127.0.0.1:8791"
	defaultFulfilment   = "127.0.0.1:8792"
)

func listen(addr string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
}

// checkAddress accepts only this machine's own address with a port: a fake answers every key and keeps whatever it is
// told, so it never listens where another machine can reach it.
func checkAddress(name, addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("--%s %q: want host:port with a port (for example 127.0.0.1:0): %w", name, addr, err)
	}

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("--%s %q: %q is not a port", name, addr, port)
	}

	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("--%s %q: a fake listens only on this machine, at 127.0.0.1", name, addr)
	}

	return nil
}

// run is the whole program; main only connects it to the process. It returns the exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, listenOn func(addr string) (net.Listener, error)) int {
	flags := flag.NewFlagSet("wyofakes", flag.ContinueOnError)
	flags.SetOutput(stderr)

	mp := flags.String("marketplaces", defaultMarketplaces, "address of the fake marketplaces, wildberries, ozon and yandex, on one address (127.0.0.1:0 picks a free port)")
	fc := flags.String("fulfilment", defaultFulfilment, "address of the fake fulfilment centre (127.0.0.1:0 picks a free port)")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}

		return exitBadInput
	}

	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "wyofakes: unexpected argument %q\n", flags.Arg(0))

		return exitBadInput
	}

	for name, addr := range map[string]string{"marketplaces": *mp, "fulfilment": *fc} {
		if err := checkAddress(name, addr); err != nil {
			_, _ = fmt.Fprintf(stderr, "wyofakes: %v\n", err)

			return exitBadInput
		}
	}

	if *mp == *fc && *mp != "127.0.0.1:0" {
		_, _ = fmt.Fprintln(stderr, "wyofakes: --marketplaces and --fulfilment are the same address")

		return exitBadInput
	}

	_ = stdout

	log := logging.New(stderr, slog.LevelInfo)

	mpLn, err := listenOn(*mp)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wyofakes: cannot listen on %s: %v\n", *mp, err)

		return exitFailure
	}

	fcLn, err := listenOn(*fc)
	if err != nil {
		_ = mpLn.Close()

		_, _ = fmt.Fprintf(stderr, "wyofakes: cannot listen on %s: %v\n", *fc, err)

		return exitFailure
	}

	fulfilmentFake := fulfilment.New(fulfilment.Options{Clock: clock.Real{}})
	defer fulfilmentFake.Close()

	wildberriesFake := wildberries.New(wildberries.Options{Clock: clock.Real{}})
	ozonFake := ozon.New(ozon.Options{Clock: clock.Real{}})
	yandexFake := yandex.New(yandex.Options{Clock: clock.Real{}})

	return serveBoth(ctx, log, []fake{
		{name: "marketplaces", ln: mpLn, handler: fakekit.HostRouter(wildberriesFake.Handler(), ozonFake, yandexFake)},
		{name: "fulfilment", ln: fcLn, handler: fulfilmentFake.Handler()},
	})
}

type fake struct {
	name    string
	ln      net.Listener
	handler http.Handler
}

// serveBoth serves every fake until the context ends or one of them fails; then all stop.
func serveBoth(ctx context.Context, log *slog.Logger, fakes []fake) int {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed bool
	)

	for _, f := range fakes {
		log.Info("listening", "fake", f.name, "address", f.ln.Addr().String())

		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := server.Serve(ctx, f.ln, f.handler, log.With("fake", f.name)); err != nil {
				log.Error("stopped with a failure", "fake", f.name, "err", err)

				mu.Lock()
				failed = true
				mu.Unlock()

				stop()
			}
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if failed {
		return exitFailure
	}

	log.Info("stopped")

	return exitOK
}
