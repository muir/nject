package nvelope

import (
	"strings"

	"github.com/muir/nject/nject"
)

var DebugIncludeExclude = nject.Required(nject.Provide("debug-include/exclude",
	func(log BasicLogger, d *nject.Debugging) {
		log.Debug(strings.Join(d.IncludeExclude, "\n"))
	}))
