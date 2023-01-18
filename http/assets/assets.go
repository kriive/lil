package assets

import (
	"embed"
	"io/fs"
)

//go:embed css
//go:embed images
var fsys embed.FS

var FS fs.FS = fsys
