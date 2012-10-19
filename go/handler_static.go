// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"net/http"
	"path/filepath"
)

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	if !PathExists(filePath) {
		logger.Noticef("serveFileFromDir() file '%s' doesn't exist, referer: '%s'", fileName, getReferer(r))
	}
	http.ServeFile(w, r, filePath)
}

// url: /s/*
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[len("/s/"):]
	serveFileFromDir(w, r, "static", file)
}

// url: /img/*
func handleStaticImg(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[len("/img/"):]
	serveFileFromDir(w, r, "img", file)
}

// url: /robots.txt
func handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	serveFileFromDir(w, r, "static", "robots.txt")
}
