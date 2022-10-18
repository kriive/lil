package html

import "embed"

//go:embed ui/partials
//go:embed ui/views
//go:embed ui/base.tmpl.html
var FS embed.FS
