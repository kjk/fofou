package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
)

type ModelForum struct {
	Forum
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
}

// handler for url: /{forum}
func handleForum(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	siteUrl := vars["forum"]
	forum := findForum(siteUrl)
	if nil == forum {
		serveErrorMsg(w, fmt.Sprintf("Forum \"%s\" doesn't exist", siteUrl))
		return
	}

	user := decodeUserFromCookie(r)
	model := &ModelForum{
		Forum:       *forum,
		User:        user,
		UserIsAdmin: false,
		RedirectUrl: r.URL.String()}

	if err := GetTemplates().ExecuteTemplate(w, tmplForum, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
