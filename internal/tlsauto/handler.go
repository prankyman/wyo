package tlsauto

import (
	"net"
	"net/http"
	"net/netip"
)

// portEightyHandler answers what arrives on port 80: the certificate authority's challenge (the one thing that must
// stay plain http), the wyo server's own health calls from this machine (so the updater and the installer can ask
// without a certificate), and a redirect to https for everything else. The redirect always goes to the configured
// domain, never to the name the visitor wrote, so the Host header cannot send anybody elsewhere.
func portEightyHandler(domain, httpsPort string, main http.Handler, challenge func(http.ResponseWriter, *http.Request) bool) http.Handler {
	target := domain
	if httpsPort != "" && httpsPort != "443" {
		target = net.JoinHostPort(domain, httpsPort)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if challenge(w, r) {
			return
		}

		if isLocalHealthCall(r) {
			main.ServeHTTP(w, r)

			return
		}

		w.Header().Set("Location", "https://"+target+r.URL.RequestURI())
		w.WriteHeader(http.StatusPermanentRedirect)
	})
}

func isLocalHealthCall(r *http.Request) bool {
	if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
		return false
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}

	address, err := netip.ParseAddr(host)

	return err == nil && address.IsLoopback()
}
