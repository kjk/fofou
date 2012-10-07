// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"net/http"
)

// url: /{forum}/rss
func handleRss(w http.ResponseWriter, r *http.Request) {
	serveErrorMsg(w, "Not implemented yet")
}
