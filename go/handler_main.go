// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"net/http"
)

type ModelMain struct {
	Forums        *[]*Forum
	AnalyticsCode string
}

// url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	if !isTopLevelUrl(r.URL.Path) {
		serve404(w, r)
		return
	}

	model := &ModelMain{
		Forums:        &appState.Forums,
		AnalyticsCode: *config.AnalyticsCode,
	}
	ExecTemplate(w, tmplMain, model)
}
