// Package datadir names the files of the wyo server's data folder, so that no other package writes a file name of its
// own. The folder holds what the seller must keep: the database, the key file, the certificates and the copies.
package datadir

import (
	"fmt"
	"os"
	"path/filepath"
)

// folderMode lets the system user of the wyo server in and its group read; nobody else.
const folderMode = 0o750

// Layout is one data folder.
type Layout struct {
	Dir string
}

// Database is the SQLite database.
func (l Layout) Database() string { return filepath.Join(l.Dir, "wyo.db") }

// KeyFile is the file with the key that encrypts the stored secrets.
func (l Layout) KeyFile() string { return filepath.Join(l.Dir, "secret.key") }

// Certificates is the folder of the certificate and its key.
func (l Layout) Certificates() string { return filepath.Join(l.Dir, "certs") }

// Backups is the folder of the copies of the database.
func (l Layout) Backups() string { return filepath.Join(l.Dir, "backups") }

// LockFile is the file the running wyo server holds a lock on, so that a copy is never put back under it.
func (l Layout) LockFile() string { return filepath.Join(l.Dir, "wyo.lock") }

// Ensure makes the data folder and its two folders when they are missing, and changes nothing that exists.
func (l Layout) Ensure() error {
	for _, dir := range []string{l.Dir, l.Backups(), l.Certificates()} {
		if err := os.MkdirAll(dir, folderMode); err != nil {
			return fmt.Errorf("make the folder %s: %w", dir, err)
		}
	}

	return nil
}
