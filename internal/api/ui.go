package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiFiles embed.FS

func uiHandler() http.Handler {
	root, _ := fs.Sub(uiFiles, "ui")
	return http.FileServer(http.FS(root))
}
