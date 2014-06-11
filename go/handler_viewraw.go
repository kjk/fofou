// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// url: /{forum}/viewraw?topicId=${topicId}&postId=${postId}
func handleViewRaw(w http.ResponseWriter, r *http.Request) {
	forum, topicId, postId := getTopicAndPostId(w, r)
	if 0 == topicId {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		logger.Noticef("handleViewRaw(): didn't find topic with id %d, referer: %q", topicId, getReferer(r))
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}
	post := topic.Posts[postId-1]
	w.Header().Set("Content-Type", "text/plain")
	sha1 := post.MessageSha1
	msgFilePath := forum.Store.MessageFilePath(sha1)
	msg, _ := ioutil.ReadFile(msgFilePath)
	w.Write([]byte("****** Raw:\n"))
	w.Write(msg)
	w.Write([]byte("\n\n****** Converted:\n"))
	w.Write([]byte(msgToHtml(string(msg))))
}
