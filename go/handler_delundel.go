// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func getTopicAndPostId(w http.ResponseWriter, r *http.Request) (*Forum, int, int) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("getTopicAndPostId(): didn't find forum\n")
		return forum, 0, 0
	}
	topicIdStr := strings.TrimSpace(r.FormValue("topicId"))
	postIdStr := strings.TrimSpace(r.FormValue("postId"))
	topicId, err := strconv.Atoi(topicIdStr)
	if err != nil {
		return forum, 0, 0
	}
	postId, err := strconv.Atoi(postIdStr)
	if err != nil {
		return forum, 0, 0
	}
	return forum, topicId, postId
}

// handler for url: /{forum}/postdel?topicId=${topicId}&postId=${postId}
func handlePostDelete(w http.ResponseWriter, r *http.Request) {
	forum, topicId, postId := getTopicAndPostId(w, r)
	if 0 == topicId {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}
	//fmt.Printf("handlePostDelete(): forum: '%s', topicId: %d, postId: %d\n", forum.ForumUrl, topicId, postId)
	// TODO: handle error?
	_ = forum.Store.DeletePost(topicId, postId)
	http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, topicId), 302)
}

// handler for url: /{forum}/postundel?topicId=${topicId}&postId=${postId}
func handlePostUndelete(w http.ResponseWriter, r *http.Request) {
	forum, topicId, postId := getTopicAndPostId(w, r)
	if 0 == topicId {
		http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, topicId), 302)
		return
	}
	//fmt.Printf("handlePostUndelete(): forum: '%s', topicId: %d, postId: %d\n", forum.ForumUrl, topicId, postId)
	// TODO: handle error?
	_ = forum.Store.UndeletePost(topicId, postId)
	http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, topicId), 302)
}
