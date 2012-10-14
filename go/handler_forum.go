// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
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
	User          string
	UserIsAdmin   bool
	ErrorMsg      string
	RedirectUrl   string
	SidebarHtml   template.HTML
	NewFrom       int
	Topics        []*TopicDisplay
	AnalyticsCode *string
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
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleForum(): didn't find forum\n")
		serveErrorMsg(w, fmt.Sprintf("Forum \"%s\" doesn't exist", forumUrl))
		return
	}
	fromStr := strings.TrimSpace(r.FormValue("from"))
	from := 0
	if "" != fromStr {
		var err error
		if from, err = strconv.Atoi(fromStr); err != nil {
			from = 0
		}
	}
	//fmt.Printf("handleForum(): forum: '%s', from: %d\n", forumUrl, from)

	nTopicsMax := 50
	user := decodeUserFromCookie(r)
	isAdmin := userIsAdmin(forum, user)
	withDeleted := isAdmin
	topics, newFrom := forum.Store.GetTopics(nTopicsMax, from, withDeleted)
	topicsDisplay := make([]*TopicDisplay, len(topics), len(topics))

	for i, t := range topics {
		d := &TopicDisplay{
			Topic:     *t,
			CreatedBy: t.Posts[0].UserName,
		}
		nComments := len(t.Posts)
		if 0 == i {
			d.CommentsCountMsg = plural(nComments, "comment")
		} else {
			d.CommentsCountMsg = fmt.Sprintf("%d", nComments)
		}
		if t.IsDeleted() {
			d.TopicLinkClass = "deleted"
		}
		if 0 == nComments {
			d.TopicUrl = fmt.Sprintf("/%s/topic?id=%d", forumUrl, t.Id)
		} else {
			d.TopicUrl = fmt.Sprintf("/%s/topic?id=%d&comments=%d", forumUrl, t.Id, nComments)
		}
		topicsDisplay[i] = d
	}

	model := &ModelForum{
		Forum:         *forum,
		User:          user,
		UserIsAdmin:   isAdmin,
		RedirectUrl:   r.URL.String(),
		Topics:        topicsDisplay,
		SidebarHtml:   template.HTML(forum.Sidebar),
		NewFrom:       newFrom,
		AnalyticsCode: config.AnalyticsCode,
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplForum, model); err != nil {
		fmt.Printf("handleForum(): ExecuteTemplate error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
