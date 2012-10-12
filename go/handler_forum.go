// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"net/http"
)

type TopicDisplay struct {
	Topic
	CommentsCountMsg string
	CreatedBy        string
	TopicLinkClass   string
	TopicUrl         string
}

type ModelForum struct {
	Forum
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
	SidebarHtml template.HTML
	NewFrom     int
	Topics      []*TopicDisplay
}

func plural(n int, s string) string {
	if 1 == n {
		return fmt.Sprintf("%d %s", n, s)
	}
	return fmt.Sprintf("%d %ss", n, s)
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
	topicsDisplay := make([]*TopicDisplay, n, n)
	for idx, t := range topics {
		d := &TopicDisplay{
			Topic:     *t,
			CreatedBy: t.Posts[0].UserName,
		}
		nComments := len(t.Posts)
		if 0 == idx {
			d.CommentsCountMsg = plural(nComments, "comment")
		} else {
			d.CommentsCountMsg = fmt.Sprintf("%d", nComments)
		}
		if t.IsDeleted {
			d.TopicLinkClass = "deleted"
		}
		if 0 == nComments {
			d.TopicUrl = fmt.Sprintf("/%s/topic?id=%d", forumUrl, t.Id)
		} else {
			d.TopicUrl = fmt.Sprintf("/%s/topic?id=%d&comments=%d", forumUrl, t.Id, nComments)
		}
		topicsDisplay[idx] = d
	}

	// TODO: set newFrom to 0 if there are no more topics after those
	newFrom := len(topicsDisplay) + from

	model := &ModelForum{
		Forum:       *forum,
		User:        user,
		UserIsAdmin: false,
		RedirectUrl: r.URL.String(),
		Topics:      topicsDisplay,
		SidebarHtml: template.HTML(forum.Sidebar),
		NewFrom:     newFrom,
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplForum, model); err != nil {
		fmt.Printf("handleForum(): Execute template error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
