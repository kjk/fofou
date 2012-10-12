// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
)

type DisplayTopic struct {
	Topic
	CommentsCount int
	MsgShort      string
	CreatedBy     string
}

type ModelForum struct {
	Forum
	NewFrom     int
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
	FromNext    int
	Topics      []*DisplayTopic
}

// handler for url: /{forum}
func handleForum(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	fmt.Printf("handleForum(): forum: '%s'\n", forumUrl)
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleForum(): didn't find forum\n")
		serveErrorMsg(w, fmt.Sprintf("Forum \"%s\" doesn't exist", forumUrl))
		return
	}
	// TODO: if it's /{forum}/?from=${from}, extract ${from}
	from := 0

	nTopicsMax := 75
	user := decodeUserFromCookie(r)
	topics := forum.Store.GetTopics(nTopicsMax, from)
	n := len(topics)
	displayTopics := make([]*DisplayTopic, n, n)
	for idx, t := range topics {
		displayTopic := &DisplayTopic{
			Topic:     *t,
			CreatedBy: t.Posts[0].UserName,
		}
		displayTopic.MsgShort = "hello"
		displayTopics[idx] = displayTopic
	}

	model := &ModelForum{
		Forum:       *forum,
		User:        user,
		UserIsAdmin: false,
		RedirectUrl: r.URL.String(),
		Topics:      displayTopics,
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplForum, model); err != nil {
		fmt.Printf("handleForum(): Execute template error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
