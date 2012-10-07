// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
)

type ModelForum struct {
	Forum
	NewFrom     int
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
}

// handler for url: /{forum}
func handleForum(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		serveErrorMsg(w, fmt.Sprintf("Forum \"%s\" doesn't exist", forumUrl))
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
	}
}
