// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"net/http"
	_ "strings"
)

// handler for url: /{forum}/postsBy?[userInternal=${userInternal}][ipIternal=${ipInternal}]
func handlePostsBy(w http.ResponseWriter, r *http.Request) {
	_, forum := mustGetForum(w, r)
	if forum == nil {
		return
	}

	/*
		userInternal := strings.TrimSpace(r.FormValue("userInternal"))
		ipInternal := strings.TrimSpace(r.FormValue("ipInternal"))
	*/
}
