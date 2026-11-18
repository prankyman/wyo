package release

import (
	"fmt"
	"strconv"
	"strings"
)

// Newer says whether the candidate version is later than the installed one. Versions are three numbers, major.minor.patch,
// with an optional "v" in front; anything else (a development build's "dev", a pre-release text) is an error, because
// the updater must not guess what is later. The updater therefore never goes back to an older version.
func Newer(candidate, installed string) (bool, error) {
	later, err := parseVersion(candidate)
	if err != nil {
		return false, fmt.Errorf("the release's version: %w", err)
	}

	current, err := parseVersion(installed)
	if err != nil {
		return false, fmt.Errorf("the installed version: %w", err)
	}

	for part := range later {
		if later[part] != current[part] {
			return later[part] > current[part], nil
		}
	}

	return false, nil
}

func parseVersion(text string) ([3]int, error) {
	var version [3]int

	fields := strings.Split(strings.TrimPrefix(strings.TrimSpace(text), "v"), ".")
	if len(fields) != len(version) {
		return version, fmt.Errorf("%q is not three numbers separated by dots", strings.TrimSpace(text))
	}

	for part, field := range fields {
		number, err := strconv.ParseUint(field, 10, 31)
		if err != nil {
			return version, fmt.Errorf("%q is not three numbers separated by dots", strings.TrimSpace(text))
		}

		version[part] = int(number)
	}

	return version, nil
}
