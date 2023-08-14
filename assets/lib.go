package assets

import (
	"embed"
	"net/http"
)

//go:embed *.css
var fs embed.FS

var FileServer http.Handler

func init() {
	server := http.FileServer(http.FS(fs))
	FileServer = http.StripPrefix("/assets", server)
}
