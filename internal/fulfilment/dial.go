package fulfilment

import (
	"github.com/prankyman/wyo/internal/logging"
	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/sync"
)

// Dialer makes clients whose every request goes through the one caller with the outbound gate's client of the
// fulfilment centre's purpose.
type Dialer struct {
	Caller *sync.Caller
	// HTTP is the outbound gate's client of the fulfilment centre; it refuses every address but the one in the setting.
	HTTP marketplace.Doer
	// AllowHTTP accepts a plain http address: only a build for development and tests sets it, for the fake.
	AllowHTTP bool
}

// Open is the client of the saved setting: the key is read from the secret store under secretName at every call.
func (d Dialer) Open(address, secretName, job string) (*Client, error) {
	return d.client(address, sync.Target{Job: job, SecretName: secretName}, d.AllowHTTP)
}

// Probe is the client of an address and a key typed by the owner and not saved yet.
func (d Dialer) Probe(address string, key logging.Secret, job string) (*Client, error) {
	return d.client(address, sync.Target{Job: job, Key: key}, d.AllowHTTP)
}

func (d Dialer) client(text string, target sync.Target, allowHTTP bool) (*Client, error) {
	address, err := ParseAddress(text, allowHTTP)
	if err != nil {
		return nil, err
	}

	target.Adapter = NewEndpoint(address)
	target.Client = d.HTTP

	return NewClient(address, d.Caller.Conn(target).Doer)
}
