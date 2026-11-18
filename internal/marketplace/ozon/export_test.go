package ozon

import (
	"context"

	"github.com/prankyman/wyo/internal/marketplace"
)

// The adapter's own tables and its one function of refusals, for its tests (package ozon_test).
var (
	RefusalOf     = refusalOf
	WriteRefusal  = writeRefusal
	FBSStates     = fbsStates
	FBOStates     = fboStates
	RolesAllow    = rolesAllow
	FileOf        = fileOf
	KindPDF       = kindPDF
	KindPNG       = kindPNG
	MaxFileBytes  = maxFileBytes
	TaskTypeOf    = taskTypeOf
	StatusApprove = approved
	PartsOf       = partsOf
)

// FileKind is a kind of file the adapter takes.
type FileKind = fileKind

// FetchFile fetches a label's link as ReadLabels does, for the tests of what is refused before a request leaves.
func (a *Adapter) FetchFile(ctx context.Context, conn marketplace.Conn, link string) ([]byte, error) {
	return a.fetchFile(ctx, conn, link, kindPDF)
}
