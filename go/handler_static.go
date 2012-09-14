package main

import (
	"fmt"
	"net/http"
	"path/filepath"
)

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	if !FileExists(filePath) {
		fmt.Printf("serveFileFromDir() file=%s doesn't exist\n", filePath)
	}
	http.ServeFile(w, r, filePath)
}

func serveFileStatic(w http.ResponseWriter, r *http.Request, fileName string) {
	serveFileFromDir(w, r, staticDir, fileName)
}

const lenStatic = len("/s/")

// handler for url: /s/
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[lenStatic:]
	serveFileStatic(w, r, file)
}
