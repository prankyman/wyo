//go:build !wyotest

package tlsauto

import (
	"crypto/x509"
	"errors"
	"testing"

	"github.com/prankyman/wyo/internal/outbound"
)

func TestAReleaseBuildRefusesTestPortsAndRoots(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		change func(*Options)
	}{
		{name: "another port for http", change: func(o *Options) { o.HTTPAddr = ":8080" }},
		{name: "another port for https", change: func(o *Options) { o.HTTPSAddr = ":8443" }},
		{name: "other roots for the authority", change: func(o *Options) { o.AuthorityRoots = x509.NewCertPool() }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := validOptions(t)
			tc.change(&opts)

			if _, err := New(opts); !errors.Is(err, outbound.ErrNotATestBuild) {
				t.Fatalf("New error = %v, want ErrNotATestBuild", err)
			}
		})
	}
}
