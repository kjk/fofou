// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type ModelTopic struct {
	Forum
	Topic
	SidebarHtml  template.HTML
	ForumUrl     string
	Posts        []*PostDisplay
	IsModerator  bool
	CreatedOnStr string
}

type PostDisplay struct {
	Post
	Id           int
	UserIpStr    string
	UserHomepage string
	MessageHtml  template.HTML
	CssClass     string
}

// handler for url: /{forum}/topic?id=${id}
func handleTopic(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleTopic(): didn't find forum\n")
		http.Redirect(w, r, "/", 302)
		return
	}
	idStr := strings.TrimSpace(r.FormValue("id"))
	topicId, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	fmt.Printf("handleTopic(): forum: '%s', topicId: %d\n", forumUrl, topicId)
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		fmt.Printf("Didn't find topic with id %d\n", topicId)
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}
	posts := make([]*PostDisplay, 0)
	for idx, p := range topic.Posts {
		pd := &PostDisplay{
			Post:     p,
			Id:       idx + 0,
			CssClass: "post",
		}
		if pd.IsDeleted {
			pd.CssClass = "post deleted"
		}
		sha1 := p.MessageSha1
		msgFilePath := forum.Store.MessageFilePath(sha1)
		msg, err := ioutil.ReadFile(msgFilePath)
		msgStr := ""
		if err != nil {
			msgStr = fmt.Sprintf("Error: failed to fetch a message with sha1 %x, file: %s", sha1[:], msgFilePath)
		} else {
			// TODO: auto-urlize links
			msgStr = string(msg)
		}
		pd.MessageHtml = template.HTML(template.HTMLEscapeString(msgStr))
		posts = append(posts, pd)
	}

	model := &ModelTopic{
		Forum:       *forum,
		Topic:       *topic,
		SidebarHtml: template.HTML(forum.Sidebar),
		ForumUrl:    forumUrl,
		Posts:       posts,
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplTopic, model); err != nil {
		fmt.Printf("handleForum(): Execute template error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
