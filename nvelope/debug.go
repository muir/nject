package nvelope

import (
	"strings"

	"github.com/muir/nject/nject"
)

// DebugIncludeExclude is a tiny wrapper around nject.Debugging.
// It logs the IncludeExclude strings.
var DebugIncludeExclude = nject.Required(nject.Provide("debug-include/exclude",
	func(log BasicLogger, d *nject.Debugging) {
		log.Debug(strings.Join(d.IncludeExclude, "\n"))
	}))
