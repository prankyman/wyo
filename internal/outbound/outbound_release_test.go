//go:build !wyotest

package outbound_test

import (
	"crypto/x509"
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func TestAReleaseBuildRefusesTestAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts outbound.Options
	}{
		{name: "another release address", opts: outbound.Options{ReleaseURL: "http://127.0.0.1:9999/"}},
		{name: "another certificate authority", opts: outbound.Options{AuthorityDirectory: "https://127.0.0.1:14000/dir"}},
		{name: "another local address", opts: outbound.Options{LocalOrigin: "http://127.0.0.1:9999"}},
		{name: "other roots for the authority", opts: outbound.Options{AuthorityRoots: x509.NewCertPool()}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := outbound.New(tc.opts); !errors.Is(err, outbound.ErrNotATestBuild) {
				t.Fatalf("New = %v, want ErrNotATestBuild", err)
			}
		})
	}
}
