package yandex

import (
	"bytes"
	"io"
	"net/http"
)

// maxFileBytes is the largest file the wyo server takes from Яндекс Маркет, a label or an act: 5 MB, the limit of the neutral API
// of the fulfilment centre it passes the file on to (docs/fulfilment-centre-api.md).
const maxFileBytes = 5 * 1024 * 1024

// maxErrorBytes is the most the adapter reads of the answer of a refused call, for Яндекс Маркет's words.
const maxErrorBytes = 64 * 1024

// pdfType is the type of the files Яндекс Маркет gives: the label of a box and the act of a shipment are PDF files.
const pdfType = "application/pdf"

var pdfMagic = []byte("%PDF")

// pdfOf is THE one function that judges a file Яндекс Маркет gave: it reads no more than 5 MB and one byte, and passes the file on
// only when it begins as a PDF, whatever the answer said it was. A file over 5 MB is refused as too large, and anything that is not
// a PDF is refused as not a file of the kind.
func pdfOf(body io.Reader) ([]byte, error) {
	// One byte more than the limit is read, so that a file over it is told from one that is exactly at it.
	raw, err := io.ReadAll(io.LimitReader(body, maxFileBytes+1))
	if err != nil {
		return nil, err
	}

	switch {
	case len(raw) > maxFileBytes:
		return nil, ownRefusal(codeFileTooLarge, map[string]any{"limit": maxFileBytes})
	case !bytes.HasPrefix(raw, pdfMagic):
		return nil, ownRefusal(codeFileNotAFile, map[string]any{"type": pdfType})
	}

	return raw, nil
}

// fileOf reads the answer of a method that gives a file: a refused call is turned into a refusal with Яндекс Маркет's words (read at
// most 64 KB), and a file is judged by pdfOf. The caller closes the answer's body.
func fileOf(res *http.Response) ([]byte, error) {
	if res.StatusCode != http.StatusOK {
		words, err := io.ReadAll(io.LimitReader(res.Body, maxErrorBytes))
		if err != nil {
			return nil, err
		}

		return nil, refusalOf(res.StatusCode, words, categoryOrdersWrite)
	}

	return pdfOf(res.Body)
}
