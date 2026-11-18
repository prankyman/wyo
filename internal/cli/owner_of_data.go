package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// becomeDataOwner makes a command that root runs (`sudo wyo backup`) work as the user that owns the data folder, so that
// every file it writes belongs to the wyo server, which runs as that user and must be able to read and write it. A command
// run by that user already, or on a folder root owns, or on a folder that does not exist yet, changes nothing.
func becomeDataOwner(sys System, dir string) error {
	if sys.Euid() != 0 {
		return nil
	}

	uid, gid, err := sys.OwnerOf(dir)
	if err != nil || uid == 0 {
		return nil //nolint:nilerr // a folder that does not exist yet is made by the command itself, as root asked.
	}

	return sys.BecomeUser(uid, gid)
}

// giveBackToDataOwner gives the database and the copies back to the owner of the data folder after the updater, which
// runs as root, wrote or put back one of them: otherwise the wyo server could not open its own database.
func giveBackToDataOwner(sys System, dir string) error {
	uid, gid, err := sys.OwnerOf(dir)
	if err != nil {
		return err
	}

	paths, err := filepath.Glob(filepath.Join(dir, "wyo.db*"))
	if err != nil {
		return err //nolint:wrapcheck // the pattern is fixed; Glob fails only on a bad pattern.
	}

	copies, err := filepath.Glob(filepath.Join(dir, "backups", "*"))
	if err != nil {
		return err //nolint:wrapcheck // as above.
	}

	for _, path := range append(paths, copies...) {
		if strings.HasSuffix(path, ".lock") {
			continue
		}

		if err := sys.Chown(path, uid, gid); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}
