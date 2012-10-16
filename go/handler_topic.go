// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ModelTopic struct {
	Forum
	Topic
	SidebarHtml   template.HTML
	ForumUrl      string
	Posts         []*PostDisplay
	IsAdmin       bool
	AnalyticsCode *string
	LogInOut      template.HTML
}

type PostDisplay struct {
	Post
	Id           int
	UserIpStr    string
	UserHomepage string
	CreatedOnStr string
	MessageHtml  template.HTML
	CssClass     string
}

func formatTime(t time.Time) string {
	s := t.Format("January 2, 2006")
	return s
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

		// placeHolder is meant to be an unlikely string
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

// handler for url: /{forum}/topic?id=${id}
func handleTopic(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		logger.Noticef("handleTopic(): didn't find forum\n")
		http.Redirect(w, r, "/", 302)
		return
	}
	idStr := strings.TrimSpace(r.FormValue("id"))
	topicId, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	//fmt.Printf("handleTopic(): forum: '%s', topicId: %d\n", forumUrl, topicId)
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		logger.Noticef("handleTopic(): didn't find topic with id %d\n", topicId)
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	isAdmin := userIsAdmin(forum, getSecureCookie(r))
	if topic.IsDeleted() && !isAdmin {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
		return
	}

	posts := make([]*PostDisplay, 0)
	for idx, p := range topic.Posts {
		pd := &PostDisplay{
			Post:         p,
			Id:           idx + 1,
			CssClass:     "post",
			CreatedOnStr: formatTime(p.CreatedOn),
		}
		if pd.IsDeleted {
			if !isAdmin {
				continue
			}
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
		posts = append(posts, pd)
	}

	model := &ModelTopic{
		Forum:         *forum,
		Topic:         *topic,
		SidebarHtml:   template.HTML(forum.Sidebar),
		ForumUrl:      forumUrl,
		Posts:         posts,
		AnalyticsCode: config.AnalyticsCode,
		IsAdmin:       isAdmin,
		LogInOut:      getLogInOut(r, getSecureCookie(r)),
	}
	ExecTemplate(w, tmplTopic, model)
}
