package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/prankyman/wyo/internal/outbound"
)

// Download writes the file at address to dest. It writes nothing that is larger than limit bytes, and leaves no file
// behind when it fails. The caller's client decides where the request may go (internal/outbound).
func Download(ctx context.Context, doer outbound.Doer, address, dest string, limit int64) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", fileNameOf(address), err)
	}

	res, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", fileNameOf(address), err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: the server answered %d", fileNameOf(address), res.StatusCode)
	}

	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: dest is a path the caller chose.
	if err != nil {
		return fmt.Errorf("download %s: %w", fileNameOf(address), err)
	}

	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("download %s: %w", fileNameOf(address), closeErr)
		}

		if err != nil {
			_ = os.Remove(dest)
		}
	}()

	written, err := io.Copy(file, io.LimitReader(res.Body, limit+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", fileNameOf(address), err)
	}

	if written > limit {
		return errors.New("download " + fileNameOf(address) + ": the file is larger than " + fmt.Sprint(limit) + " bytes")
	}

	return nil
}

// fileNameOf is the last part of an address, so that an error names the file and never a query.
func fileNameOf(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == '/' {
			return address[i+1:]
		}
	}

	return address
}
