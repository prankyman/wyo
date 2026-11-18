package ozon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/prankyman/wyo/internal/marketplace"
)

// maxFileBytes is the largest file the wyo server takes from Ozon, a label or a hand-over document: 5 MB, the limit of the
// neutral API of the fulfilment centre it passes the file on to (docs/fulfilment-centre-api.md).
const maxFileBytes = 5 * 1024 * 1024

// fileKind is a kind of file the adapter takes, known by how its first bytes begin: a file that does not begin so is never
// passed on, whatever the answer said it was.
type fileKind struct {
	contentType string
	magic       []byte
}

var (
	// kindPDF is a label, the shipping list and the waybill: «%PDF».
	kindPDF = fileKind{contentType: "application/pdf", magic: []byte("%PDF")}
	// kindPNG is the hand-over barcode's picture: the eight bytes every PNG begins with.
	kindPNG = fileKind{contentType: "image/png", magic: []byte("\x89PNG\r\n\x1a\n")}
)

// fileOf is THE one function that reads a file Ozon returned, for the labels and for the hand-over documents. Ozon's file
// declares the answer of a document as the file's own type with a schema of an object holding the file in base64
// («file_content», «file_name», «content_type»), and does not say which of the two the body is (OZC §9, §10 point 8). So both
// are read: the body is the file when it begins as the kind says, and otherwise it must be that object, whose file_content
// is decoded and must begin so. A file over 5 MB is refused, as is anything that is not a file of the kind.
func fileOf(raw []byte, kind fileKind) ([]byte, error) {
	file := raw

	if !bytes.HasPrefix(raw, kind.magic) {
		var object struct {
			FileContent string `json:"file_content"`
		}

		if err := json.Unmarshal(bytes.TrimSpace(raw), &object); err != nil || object.FileContent == "" {
			return nil, ownRefusal(codeFileNotAFile, "", map[string]any{"type": kind.contentType})
		}

		decoded, err := base64.StdEncoding.DecodeString(object.FileContent)
		if err != nil {
			return nil, ownRefusal(codeFileNotAFile, "", map[string]any{"type": kind.contentType})
		}

		file = decoded
	}

	switch {
	case !bytes.HasPrefix(file, kind.magic):
		return nil, ownRefusal(codeFileNotAFile, "", map[string]any{"type": kind.contentType})
	case len(file) > maxFileBytes:
		return nil, ownRefusal(codeFileTooLarge, "", map[string]any{"limit": maxFileBytes})
	}

	return file, nil
}

// fetchFile fetches the file a link of Ozon names, without the key: the request goes to the file's own route, which carries
// none (sending the key to a file link is not sourced, and a key that is not needed is never sent). The link is checked
// before anything leaves: it must be https, name a host under ozone.ru and no port, and carry no user name; any other link is
// refused with the host it named and without the link, which may hold a token. At most 5 MB are read, and what is read goes
// through fileOf, so that only a file of the kind is passed on.
func (a *Adapter) fetchFile(ctx context.Context, conn marketplace.Conn, link string, kind fileKind) ([]byte, error) {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !marketplace.HostMatches(fileHostPattern, u.Hostname()) {
		host := ""
		if err == nil {
			host = u.Hostname()
		}

		return nil, ownRefusal(codeFileLinkRefused, "", map[string]any{"host": host})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, marketplace.Origin(a.opts.TestBase, u.Hostname())+u.RequestURI(), http.NoBody)
	if err != nil {
		return nil, err
	}

	res, err := conn.Doer.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		refusal := ownRefusal(codeFileUnavailable, "", map[string]any{"status": res.StatusCode})
		refusal.Status = res.StatusCode

		return nil, refusal
	}

	// One byte more than the limit is read, so that a file over it is told from one that is exactly at it.
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxFileBytes+1))
	if err != nil {
		return nil, err
	}

	return fileOf(raw, kind)
}
