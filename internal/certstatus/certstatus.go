// Package certstatus is the shape in which the part of the wyo server that obtains its certificate reports how it
// goes. It is a package of its own so that the API and the bell read it without depending on how certificates are
// obtained.
package certstatus

import "time"

// The modes: the wyo server obtains its certificate; a web server in front holds it; plain http (development, tests).
const (
	ModeAutomatic = "automatic"
	ModeProxy     = "proxy"
	ModeNone      = "none"
)

// The states.
const (
	StateOK            = "ok"
	StatePending       = "pending"
	StateFailed        = "failed"
	StateNotApplicable = "not_applicable"
)

// The reasons a certificate could not be obtained, as codes the interface words.
const (
	ReasonDNS       = "dns"
	ReasonPorts     = "ports"
	ReasonAuthority = "authority"
	ReasonOther     = "other"
)

// Status is the certificate's state at one moment.
type Status struct {
	Mode      string
	State     string
	Domain    string
	NotAfter  *time.Time
	LastError string
}

// Reporter tells the certificate's current state.
type Reporter interface {
	Status() Status
}

// Fixed is a Reporter that always tells the same state: plain http and a web server in front never change.
type Fixed Status

// Status returns the fixed state.
func (f Fixed) Status() Status { return Status(f) }
