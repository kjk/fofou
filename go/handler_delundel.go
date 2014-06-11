// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func getTopicAndPostId(w http.ResponseWriter, r *http.Request) (*Forum, int, int) {
	forum := mustGetForum(w, r)
	if forum == nil {
		http.Redirect(w, r, "/", 302)
		return nil, 0, 0
	}
	topicIdStr := strings.TrimSpace(r.FormValue("topicId"))
	postIdStr := strings.TrimSpace(r.FormValue("postId"))
	topicId, err := strconv.Atoi(topicIdStr)
	if err != nil || topicId == 0 {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return nil, 0, 0
	}
	postId, err := strconv.Atoi(postIdStr)
	if err != nil || postId == 0 {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return forum, 0, 0
	}
	return forum, topicId, postId
}

// url: /{forum}/postdel?topicId=${topicId}&postId=${postId}
func handlePostDelete(w http.ResponseWriter, r *http.Request) {
	if forum, topicId, postId := getTopicAndPostId(w, r); forum != nil {
		//fmt.Printf("handlePostDelete(): forum: %q, topicId: %d, postId: %d\n", forum.ForumUrl, topicId, postId)
		// TODO: handle error?
		forum.Store.DeletePost(topicId, postId)
		http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, topicId), 302)
	}
}

// url: /{forum}/postundel?topicId=${topicId}&postId=${postId}
func handlePostUndelete(w http.ResponseWriter, r *http.Request) {
	if forum, topicId, postId := getTopicAndPostId(w, r); forum != nil {
		//fmt.Printf("handlePostUndelete(): forum: %q, topicId: %d, postId: %d\n", forum.ForumUrl, topicId, postId)
		// TODO: handle error?
		forum.Store.UndeletePost(topicId, postId)
		http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, topicId), 302)
	}
}
