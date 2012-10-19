// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PostDisplay struct {
	Post
	UserHomepage string
	MessageHtml  template.HTML
	CssClass     string
}

func formatPostCreatedOnTime(t time.Time) string {
	s := t.Format("January 2, 2006")
	return s
}

func (p *PostDisplay) CreatedOnStr() string {
	return formatPostCreatedOnTime(p.CreatedOn)
}

func NewPostDisplay(p *Post, forum *Forum, isAdmin bool) *PostDisplay {
	if p.IsDeleted && !isAdmin {
		return nil
	}

	pd := &PostDisplay{
		Post:     *p,
		CssClass: "post",
	}
	if p.IsDeleted {
		pd.CssClass = "post deleted"
	}
	sha1 := p.MessageSha1
	msgFilePath := forum.Store.MessageFilePath(sha1)
	msg, err := ioutil.ReadFile(msgFilePath)
	msgStr := ""
	if err != nil {
		msgStr = fmt.Sprintf("Error: failed to fetch a message with sha1 %x, file: %s", sha1[:], msgFilePath)
	} else {
		msgStr = msgToHtml(string(msg))
	}
	pd.MessageHtml = template.HTML(msgStr)

	if p.IsTwitterUser() {
		pd.UserHomepage = "http://twitter.com/" + p.UserName()
	}

	if forum.ForumUrl == "sumatrapdf" {
		// backwards-compatibility hack for posts imported from old version of
		// fofou: hyper-link my name to my website
		if p.UserName() == "Krzysztof Kowalczyk" || p.UserNameInternal == "t:kjk" {
			pd.UserHomepage = "http://blog.kowalczyk.info"
		}
	}
	return pd
}

// TODO: this is simplistic but work for me, http://net.tutsplus.com/tutorials/other/8-regular-expressions-you-should-know/
// has more elaborate regex for extracting urls
var urlRx = regexp.MustCompile(`https?://[[:^space:]]+`)
var notUrlEndChars = []byte(".),")

func notUrlEndChar(c byte) bool {
	return -1 != bytes.IndexByte(notUrlEndChars, c)
}

var disableUrlization = false

func msgToHtml(s string) string {
	matches := urlRx.FindAllStringIndex(s, -1)
	if nil == matches || disableUrlization {
		s = template.HTMLEscapeString(s)
		s = strings.Replace(s, "\n", "<br>", -1)
		return s
	}

	urlMap := make(map[string]string)
	ns := ""
	prevEnd := 0
	for n, match := range matches {
		start, end := match[0], match[1]
		for end > start && notUrlEndChar(s[end-1]) {
			end -= 1
		}
		url := s[start:end]
		ns += s[prevEnd:start]

		// placeHolder is meant to be an unlikely string that doesn't exist in
		// the message, so that we can replace the string with it and then
		// revert the replacement. A more robust approach would be to remember
		// offsets
		placeHolder, ok := urlMap[url]
		if !ok {
			placeHolder = fmt.Sprintf("a;dfsl;a__lkasjdfh1234098;lajksdf_%d", n)
			urlMap[url] = placeHolder
		}
		ns += placeHolder
		prevEnd = end
	}

	ns = template.HTMLEscapeString(ns)
	for url, placeHolder := range urlMap {
		url = fmt.Sprintf(`<a href="%s" rel="nofollow">%s</a>`, url, url)
		ns = strings.Replace(ns, placeHolder, url, -1)
	}
	ns = strings.Replace(ns, "\n", "<br>", -1)
	return ns
}

func getLogInOut(r *http.Request, c *SecureCookieValue) template.HTML {
	redirectUrl := template.HTMLEscapeString(r.URL.String())
	s := ""
	if c.TwitterUser == "" {
		s = `<span style="float: right;">Not logged in. <a href="/login?redirect=%s">Log in with Twitter</a></span>`
		s = fmt.Sprintf(s, redirectUrl)
	} else {
		s = `<span style="float:right;">Logged in as %s (<a href="/logout?redirect=%s">logout</a>)</span>`
		s = fmt.Sprintf(s, c.TwitterUser, redirectUrl)
	}
	return template.HTML(s)
}

// url: /{forum}/topic?id=${id}
func handleTopic(w http.ResponseWriter, r *http.Request) {
	forum := mustGetForum(w, r)
	if forum == nil {
		return
	}
	idStr := strings.TrimSpace(r.FormValue("id"))
	topicId, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}

	//fmt.Printf("handleTopic(): forum: '%s', topicId: %d\n", forum.ForumUrl, topicId)
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		logger.Noticef("handleTopic(): didn't find topic with id %d, referer: '%s'", topicId, getReferer(r))
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}

	isAdmin := userIsAdmin(forum, getSecureCookie(r))
	if topic.IsDeleted() && !isAdmin {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}

	posts := make([]*PostDisplay, 0)
	for _, p := range topic.Posts {
		pd := NewPostDisplay(&p, forum, isAdmin)
		if pd != nil {
			posts = append(posts, pd)
		}
	}

	model := struct {
		Forum
		Topic
		SidebarHtml   template.HTML
		Posts         []*PostDisplay
		IsAdmin       bool
		AnalyticsCode *string
		LogInOut      template.HTML
	}{
		Forum:         *forum,
		Topic:         *topic,
		SidebarHtml:   template.HTML(forum.Sidebar),
		Posts:         posts,
		IsAdmin:       isAdmin,
		AnalyticsCode: config.AnalyticsCode,
		LogInOut:      getLogInOut(r, getSecureCookie(r)),
	}
	ExecTemplate(w, tmplTopic, model)
}
