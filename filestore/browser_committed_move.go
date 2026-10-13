package filestore

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed browser_committed_move.js
var browserCommittedMoveSource string

// BrowserCommittedMoveModule emits Primitive's browser-local storage executor.
// Go's embed and strings packages publish the exact module; in the browser,
// IndexedDB owns the atomic transaction and its native terminal observations.
// Applications lend two exact keys and synchronous admission policy. The
// executor reads no history and resolves only after commit and handle close.
func BrowserCommittedMoveModule() string {
	return strings.NewReplacer(
		"__CONTRACT_IDENTITY__", strconv.Itoa(int(core.ErrFilestoreContract)),
		"__SOURCE_IDENTITY__", strconv.Itoa(int(core.ErrFilestoreSource)),
		"__DESTINATION_IDENTITY__", strconv.Itoa(int(core.ErrFilestoreDestination)),
		"__CLEANUP_IDENTITY__", strconv.Itoa(int(core.ErrFilestoreCleanup)),
		"__CALLBACK_IDENTITY__", strconv.Itoa(int(core.ErrFilestoreCallbackPanic)),
	).Replace(browserCommittedMoveSource)
}
