// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

type ModelNewPost struct {
	Forum
	SidebarHtml   template.HTML
	ForumUrl      string
	AnalyticsCode *string
	Num1          int
	Num2          int
	Num3          int
	TopicId       int
	CaptchaClass  string
	PrevCaptcha   string
	SubjectClass  string
	PrevSubject   string
	MessageClass  string
	PrevMessage   string
	NameClass     string
	PrevName      string
}

var errorClass = "error"

func isCaptchaValid(n1Str, n2Str, captchaStr string) bool {
	if n1, err := strconv.Atoi(n1Str); err != nil {
		return false
	} else if n2, err := strconv.Atoi(n2Str); err != nil {
		return false
	} else if captcha, err := strconv.Atoi(captchaStr); err != nil {
		return false
	} else {
		return captcha == n1+n2
	}
	return false
}

func isSubjectValid(subject string) bool {
	return subject != ""
}

func isNameValid(name string) bool {
	return name != ""
}

func isMsgValid(msg string, topic *Topic) bool {
	if msg == "" {
		return false
	}
	// prevent duplicate posts within the topic
	if topic != nil {
		sha1 := Sha1OfBytes([]byte(msg))
		for _, p := range topic.Posts {
			if bytes.Compare(p.MessageSha1[:], sha1) == 0 {
				return false
			}
		}
	}
	return true
}

func createNewPost(w http.ResponseWriter, r *http.Request, forumUrl string, model *ModelNewPost, topic *Topic) {
	fmt.Printf("createNewPost(): topicId=%d\n", model.TopicId)

	// validate the fields
	num1Str := strings.TrimSpace(r.FormValue("num1"))
	num2Str := strings.TrimSpace(r.FormValue("num2"))
	captchaStr := strings.TrimSpace(r.FormValue("Captcha"))
	subject := strings.TrimSpace(r.FormValue("Subject"))
	msg := strings.TrimSpace(r.FormValue("Message"))
	name := strings.TrimSpace(r.FormValue("Name"))

	model.Num1, _ = strconv.Atoi(num1Str)
	model.Num2, _ = strconv.Atoi(num2Str)
	model.Num3 = model.Num1 + model.Num2
	model.PrevCaptcha = captchaStr
	model.PrevSubject = subject
	model.PrevMessage = msg
	model.PrevName = name

	ok := true
	if !isCaptchaValid(num1Str, num2Str, captchaStr) {
		model.CaptchaClass = errorClass
		ok = false
	} else if !isSubjectValid(subject) {
		model.SubjectClass = errorClass
		ok = false
	} else if !isMsgValid(msg, topic) {
		model.MessageClass = errorClass
		ok = false
	} else if !isNameValid(name) {
		model.NameClass = errorClass
		ok = false
	}

	if !ok {
		if err := GetTemplates().ExecuteTemplate(w, tmplNewPost, model); err != nil {
			fmt.Printf("handleNewPost(): ExecuteTemplate error %s\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// TODO: create a new topic with a given post or add a post to an existing topic

	if topic == nil {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/%s/topic=%d", forumUrl, topic.Id), 302)
	}
}

// handler for url: /{forum}/newpost[?topicId={topicId}]
func handleNewPost(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	forumUrl := vars["forum"]
	forum := findForum(forumUrl)
	if nil == forum {
		fmt.Print("handleNewPost(): didn't find forum\n")
		http.Redirect(w, r, "/", 302)
		return
	}

	topicId := 0
	var topic *Topic
	topicIdStr := strings.TrimSpace(r.FormValue("topicId"))
	if topicIdStr != "" {
		if topicId, err = strconv.Atoi(topicIdStr); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
			return
		}
		if topic = forum.Store.TopicById(topicId); topic != nil {
			fmt.Printf("handleNewPost(): invalid topicId: %d\n", topicId)
			http.Redirect(w, r, fmt.Sprintf("/%s/", forumUrl), 302)
			return
		}
	}
	fmt.Printf("handleNewPost(): forum: '%s', topicId: %d\n", forumUrl, topicId)

	model := &ModelNewPost{
		Forum:         *forum,
		SidebarHtml:   template.HTML(forum.Sidebar),
		ForumUrl:      forumUrl,
		AnalyticsCode: config.AnalyticsCode,
		Num1:          rand.Intn(9) + 1,
		Num2:          rand.Intn(9) + 1,
		TopicId:       topicId,
	}
	model.Num3 = model.Num1 + model.Num2

	if r.Method == "POST" {
		createNewPost(w, r, forumUrl, model, topic)
		return
	}

	if topicId != 0 {
		model.PrevSubject = topic.Subject
	}

	if err = GetTemplates().ExecuteTemplate(w, tmplNewPost, model); err != nil {
		fmt.Printf("handleNewPost(): ExecuteTemplate error %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
