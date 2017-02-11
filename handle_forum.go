// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// TopicDisplay describes a topic
type TopicDisplay struct {
	Topic
	CommentsCountMsg string
	CreatedBy        string
	TopicLinkClass   string
	TopicURL         string
}

func plural(n int, s string) string {
	if 1 == n {
		return fmt.Sprintf("%d %s", n, s)
	}
	return fmt.Sprintf("%d %ss", n, s)
}

// those happen often so exclude them in order to not overwhelm the logs
var skipForums = []string{"fofou", "topic.php", "post", "newpost",
	"crossdomain.xml", "azenv.php", "index.php"}

func logMissingForum(forumURL, referer string) bool {
	if referer == "" {
		return false
	}
	for _, forum := range skipForums {
		if forum == forumURL {
			return false
		}
	}
	return true
}

func mustGetForum(w http.ResponseWriter, r *http.Request) *Forum {
	vars := mux.Vars(r)
	forumURL := vars["forum"]
	if forum := findForum(forumURL); forum != nil {
		return forum
	}

	if logMissingForum(forumURL, getReferer(r)) {
		logger.Noticef("didn't find forum %q, referer: %q", forumURL, getReferer(r))
	}
	httpErrorf(w, "Forum %q doesn't exist", forumURL)
	return nil
}

// url: /{forum}
func handleForum(w http.ResponseWriter, r *http.Request) {
	forum := mustGetForum(w, r)
	if forum == nil {
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
	//fmt.Printf("handleForum(): forum: %q, from: %d\n", forum.ForumUrl, from)

	nTopicsMax := 50
	cookie := getSecureCookie(r)
	isAdmin := userIsAdmin(forum, cookie)
	withDeleted := isAdmin
	topics, newFrom := forum.Store.GetTopics(nTopicsMax, from, withDeleted)
	topicsDisplay := make([]*TopicDisplay, 0)

	for i, t := range topics {
		if t.IsDeleted() && !isAdmin {
			continue
		}
		d := &TopicDisplay{
			Topic:     *t,
			CreatedBy: t.Posts[0].UserName(),
		}
		nComments := len(t.Posts) - 1
		if 0 == i {
			d.CommentsCountMsg = plural(nComments, "comment")
		} else {
			d.CommentsCountMsg = fmt.Sprintf("%d", nComments)
		}
		if t.IsDeleted() {
			d.TopicLinkClass = "deleted"
		}
		if 0 == nComments {
			d.TopicURL = fmt.Sprintf("/%s/topic?id=%d", forum.ForumUrl, t.Id)
		} else {
			d.TopicURL = fmt.Sprintf("/%s/topic?id=%d&comments=%d", forum.ForumUrl, t.Id, nComments)
		}
		topicsDisplay = append(topicsDisplay, d)
	}

	sidebar := DoSidebarTemplate(forum, isAdmin)
	model := struct {
		Forum
		ErrorMsg      string
		RedirectUrl   string
		SidebarHtml   template.HTML
		ForumFullUrl  string
		NewFrom       int
		Topics        []*TopicDisplay
		AnalyticsCode *string
		LogInOut      template.HTML
	}{
		Forum:         *forum,
		Topics:        topicsDisplay,
		SidebarHtml:   template.HTML(sidebar),
		ForumFullUrl:  buildForumURL(r, forum),
		NewFrom:       newFrom,
		AnalyticsCode: config.AnalyticsCode,
		LogInOut:      getLogInOut(r, getSecureCookie(r)),
	}

	ExecTemplate(w, tmplForum, model)
}
