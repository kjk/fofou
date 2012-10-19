// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"net/http"
)

// url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	if !isTopLevelUrl(r.URL.Path) {
		serve404(w, r)
		return
	}

	model := struct {
		Forums        *[]*Forum
		AnalyticsCode string
	}{
		Forums:        &appState.Forums,
		AnalyticsCode: *config.AnalyticsCode,
	}
	ExecTemplate(w, tmplMain, model)
}
