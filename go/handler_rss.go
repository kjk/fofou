// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"atom"
	"code.google.com/p/gorilla/mux"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func buildForumUrl(r *http.Request, forum *Forum) string {
	return fmt.Sprintf("http://%s/%s", r.Host, forum.ForumUrl)
}

func buildTopicUrl(r *http.Request, forum *Forum, topicId int) string {
	return fmt.Sprintf("http://%s/%s/topic?id=%d", r.Host, forum.ForumUrl, topicId)
}

func handleRss2(w http.ResponseWriter, r *http.Request, all bool) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleRss2(): didn't find forum\n")
		http.Redirect(w, r, "/", 302)
		return
	}
	var posts []PostTopic
	if all {
		posts = forum.Store.GetRecentPosts()
	} else {
		topics, _ := forum.Store.GetTopics(25, 0, false)
		posts = make([]PostTopic, len(topics), len(topics))
		for i, t := range topics {
			pt := PostTopic{Post: &t.Posts[0], Topic: t}
			posts[i] = pt
		}
	}

	pubTime := time.Now()
	if len(posts) > 0 {
		pubTime = posts[len(posts)-1].Post.CreatedOn
	}

	feed := &atom.Feed{
		Title:   forum.Title,
		Link:    buildForumUrl(r, forum),
		PubDate: pubTime,
	}

	for _, pt := range posts {
		sha1 := pt.Post.MessageSha1
		msgFilePath := forum.Store.MessageFilePath(sha1)
		msg, err := ioutil.ReadFile(msgFilePath)
		msgStr := ""
		if err != nil {
			msgStr = fmt.Sprintf("Error: failed to fetch a message with sha1 %x, file: %s", sha1[:], msgFilePath)
		} else {
			msgStr = msgToHtml(string(msg))
		}
		e := &atom.Entry{
			Title:       pt.Topic.Subject,
			Link:        buildTopicUrl(r, forum, pt.Topic.Id),
			Description: msgStr,
			PubDate:     pt.Post.CreatedOn}
		feed.AddEntry(e)
	}

	s, err := feed.GenXml()
	if err != nil {
		s = "Failed to generate XML feed"
	}

	w.Write([]byte(s))
}

// url: /{forum}/rss
func handleRss(w http.ResponseWriter, r *http.Request) {
	handleRss2(w, r, false)
}

// url: /{forum}/rssall
func handleRssAll(w http.ResponseWriter, r *http.Request) {
	handleRss2(w, r, true)
}
