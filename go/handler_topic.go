// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
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

//October 11th, 2012 4:13p.m.
// TODO: missing -st, -nd, -rd, -nt 
/*
function num_abbrev_str(num) {
	var len = num.length, last_char = num.charAt(len - 1), abbrev
	if (len == 2 && num.charAt(0) == '1') {
		abbrev = 'th'
	} else {
		if (last_char == '1') {
			abbrev = 'st'
		} else if (last_char == '2') {
	  		abbrev = 'nd'
		} else if (last_char == '3') {
	  		abbrev = 'rd'
		} else {
	  		abbrev = 'th'
		}
	}
	return num + abbrev
}
*/
func formatTime(t time.Time) string {
	s := t.Format("January 2, 2006")
	return s
}

// TODO: auto-urlize links
func msgToHtml(s string) string {
	s = template.HTMLEscapeString(s)
	s = strings.Replace(s, "\n", "<br>", -1)
	return s
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

	//fmt.Printf("handleTopic(): forum: '%s', topicId: %d\n", forumUrl, topicId)
	topic := forum.Store.TopicById(topicId)
	if nil == topic {
		fmt.Printf("handleTopic(): didn't find topic with id %d\n", topicId)
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
	if err := GetTemplates().ExecuteTemplate(w, tmplTopic, model); err != nil {
		fmt.Printf("handleTopic(): ExecuteTemplate error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
