// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"fmt"
	"net/http"
)

type ModelNewPost struct {
	Forum
	Topic
	SidebarHtml   template.HTML
	ForumUrl      string
	AnalyticsCode *string
}

// handler for url: /{forum}/newpost[?topicId={topicId}]
func handleNewPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleNewPost(): didn't find forum\n")
		http.Redirect(w, r, "/", 302)
		return
	}
	topicIdStr := strings.TrimSpace(r.FormValue("topicId"))
	topicId, err := strconv.Atoi(topicIdStr)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	//fmt.Printf("handleTopic(): forum: '%s', topicId: %d\n", forumUrl, topicId)
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		fmt.Printf("handleNewPost(): didn't find topic with id %d\n", topicId)
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	model := &ModelNewPost{
		Forum:         *forum,
		Topic:         *topic,
		SidebarHtml:   template.HTML(forum.Sidebar),
		ForumUrl:      forumUrl,
		AnalyticsCode: config.AnalyticsCode,
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplNewPost, model); err != nil {
		fmt.Printf("handleForum(): Execute template error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
