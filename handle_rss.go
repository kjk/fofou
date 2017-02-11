// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	atom "github.com/kjk/atomgenerator"
)

func buildForumURL(r *http.Request, forum *Forum) string {
	return fmt.Sprintf("http://%s/%s", r.Host, forum.ForumUrl)
}

func buildTopicURL(r *http.Request, forum *Forum, p *Post) string {
	return fmt.Sprintf("http://%s/%s/topic?id=%d&post=%d", r.Host, forum.ForumUrl, p.Topic.Id, p.Id)
}

func buildTopicID(r *http.Request, forum *Forum, p *Post) string {
	pubDateStr := p.CreatedOn.Format("2006-01-02")
	url := fmt.Sprintf("/%s/topic?id=%d&post=%d", forum.ForumUrl, p.Topic.Id, p.Id)
	return fmt.Sprintf("tag:%s,%s:%s", r.Host, pubDateStr, url)
}

func handleRss2(w http.ResponseWriter, r *http.Request, all bool) {
	forum := mustGetForum(w, r)
	if forum == nil {
		return
	}
	var posts []*Post
	if all {
		posts = forum.Store.GetRecentPosts(25)
	} else {
		topics, _ := forum.Store.GetTopics(25, 0, false)
		posts = make([]*Post, len(topics), len(topics))
		for i, t := range topics {
			posts[i] = &t.Posts[0]
		}
	}

	pubTime := time.Now()
	if len(posts) > 0 {
		pubTime = posts[0].CreatedOn
	}

	feed := &atom.Feed{
		Title:   forum.Title,
		Link:    buildForumURL(r, forum),
		PubDate: pubTime,
	}

	for _, p := range posts {
		sha1 := p.MessageSha1
		msgFilePath := forum.Store.MessageFilePath(sha1)
		msg, err := ioutil.ReadFile(msgFilePath)
		msgStr := ""
		if err != nil {
			msgStr = fmt.Sprintf("Error: failed to fetch a message with sha1 %x, file: %s", sha1[:], msgFilePath)
		} else {
			msgStr = msgToHtml(string(msg))
		}
		//id := fmt.Sprintf("tag:forums.fofou.org,1999:%s-topic-%d-post-%d", forum.ForumUrl, p.Topic.Id, p.Id)
		e := &atom.Entry{
			Id:      buildTopicID(r, forum, p),
			Title:   p.Topic.Subject,
			PubDate: p.CreatedOn,
			Link:    buildTopicURL(r, forum, p),
			Content: msgStr,
		}
		feed.AddEntry(e)
	}

	s, err := feed.GenXml()
	if err != nil {
		s = []byte("Failed to generate XML feed")
	}

	w.Write(s)
}

// url: /{forum}/rss
func handleRss(w http.ResponseWriter, r *http.Request) {
	handleRss2(w, r, false)
}

// url: /{forum}/rssall
func handleRssAll(w http.ResponseWriter, r *http.Request) {
	handleRss2(w, r, true)
}
