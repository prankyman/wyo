package config

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// InstallerFile is the file the installer writes what it was told into (the domain, the data folder, whether a web server
// stands in front). The service reads it through systemd; a command a person types reads it through WithFile, so that
// `sudo wyo doctor` knows the same as the running wyo server.
const InstallerFile = "/etc/wyo/wyo.env"

// WithFile is lookup with the KEY=VALUE lines of the file at path behind it: a variable of the environment wins, the
// file's fills what the environment lacks. A file that is missing or cannot be read adds nothing, because a machine without
// an installation has none and an ordinary user may not read it.
func WithFile(lookup LookupFunc, path string) LookupFunc {
	values := map[string]string{}

	if path != "" {
		if data, err := os.ReadFile(path); err == nil { //nolint:gosec // G304: the installer's fixed file, or a test's.
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				key, value, ok := strings.Cut(line, "=")

				if ok && !strings.HasPrefix(line, "#") {
					values[strings.TrimSpace(key)] = strings.TrimSpace(value)
				}
			}
		}
	}

	return func(key string) (string, bool) {
		if lookup != nil {
			if v, ok := lookup(key); ok {
				return v, true
			}
		}

		v, ok := values[key]

		return v, ok
	}
}
