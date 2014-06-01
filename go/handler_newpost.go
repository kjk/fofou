// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/kjk/u"
)

type ModelNewPost struct {
	Forum
	SidebarHtml     template.HTML
	AnalyticsCode   *string
	Num1            int
	Num2            int
	Num3            int
	TopicId         int
	CaptchaClass    string
	PrevCaptcha     string
	SubjectClass    string
	PrevSubject     string
	MessageClass    string
	PrevMessage     string
	NameClass       string
	PrevName        string
	LogInOut        template.HTML
	TwitterUserName string
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
		sha1 := u.Sha1OfBytes([]byte(msg))
		for _, p := range topic.Posts {
			if bytes.Compare(p.MessageSha1[:], sha1) == 0 {
				return false
			}
		}
	}
	return true
}

// Request.RemoteAddress contains port, which we want to remove i.e.:
// "[::1]:58292" => "[::1]"
func ipAddrFromRemoteAddr(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

func getIpAddress(r *http.Request) string {
	hdr := r.Header
	hdrRealIp := hdr.Get("X-Real-Ip")
	hdrForwardedFor := hdr.Get("X-Forwarded-For")
	if hdrRealIp == "" && hdrForwardedFor == "" {
		return ipAddrFromRemoteAddr(r.RemoteAddr)
	}
	if hdrForwardedFor != "" {
		// X-Forwarded-For is potentially a list of addresses separated with ","
		parts := strings.Split(hdrForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		// TODO: should return first non-local address
		return parts[0]
	}
	return hdrRealIp
}

var badUserHtml string = `
<html>
<head>
</head>

<body>
Internal problem 0xcc03fad detected ...
</body>
</html>
`

func isIpBlocked(forum Forum, ip string, ipInternal string) bool {
	if forum.Store.IsIpBlocked(ipInternal) {
		return true
	}
	banned := forum.BannedIps
	if banned != nil {
		for _, s := range *banned {
			// we have already checked that s is a valid regexp in addForum()
			r := regexp.MustCompile(s)
			if r.MatchString(ip) {
				return true
			}
		}
	}
	return false
}

func isMessageBlocked(forum Forum, msg string) bool {
	bannedWords := forum.BannedWords
	if bannedWords != nil {
		for _, s := range *bannedWords {
			if strings.Index(msg, s) != -1 {
				return true
			}
		}
	}
	return false
}

func createNewPost(w http.ResponseWriter, r *http.Request, model *ModelNewPost, topic *Topic) {
	ipAddr := getIpAddress(r)
	ipAddrInternal := ipAddrToInternal(ipAddr)
	if isIpBlocked(model.Forum, ipAddr, ipAddrInternal) {
		logger.Noticef("blocked a post from ip address %s (%s)", ipAddr, ipAddrInternal)
		w.Write([]byte(badUserHtml))
		return
	}

	if r.FormValue("Cancel") != "" {
		logger.Notice("Pressed cancel")
		http.Redirect(w, r, fmt.Sprintf("/%s/", model.Forum.ForumUrl), 302)
		return
	}

	// validate the fields
	num1Str := strings.TrimSpace(r.FormValue("num1"))
	num2Str := strings.TrimSpace(r.FormValue("num2"))
	captchaStr := strings.TrimSpace(r.FormValue("Captcha"))
	subject := strings.TrimSpace(r.FormValue("Subject"))
	msg := strings.TrimSpace(r.FormValue("Message"))
	name := strings.TrimSpace(r.FormValue("Name"))

	if isMessageBlocked(model.Forum, msg) {
		logger.Notice("blocked a post because has a banned word in it")
		w.Write([]byte(badUserHtml))
		return
	}

	model.Num1, _ = strconv.Atoi(num1Str)
	model.Num2, _ = strconv.Atoi(num2Str)
	model.Num3 = model.Num1 + model.Num2
	model.PrevCaptcha = captchaStr
	model.PrevSubject = subject
	model.PrevMessage = msg
	model.PrevName = name

	if model.TopicId != 0 {
		model.PrevSubject = topic.Subject
	}

	ok := true
	if !isCaptchaValid(num1Str, num2Str, captchaStr) {
		model.CaptchaClass = errorClass
		ok = false
	} else if (model.TopicId == 0) && !isSubjectValid(subject) {
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
		ExecTemplate(w, tmplNewPost, model)
		return
	}

	cookie := getSecureCookie(r)
	cookie.AnonUser = name
	setSecureCookie(w, cookie)

	userName := cookie.TwitterUser
	twitterUser := true
	if userName == "" {
		userName = cookie.AnonUser
		twitterUser = false
	}
	userName = MakeInternalUserName(userName, twitterUser)

	store := model.Forum.Store
	if topic == nil {
		if topicId, err := store.CreateNewPost(subject, msg, userName, ipAddr); err != nil {
			logger.Errorf("createNewPost(): store.CreateNewPost() failed with %s", err)
		} else {
			http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", model.ForumUrl, topicId), 302)
		}
	} else {
		if err := store.AddPostToTopic(topic.Id, msg, userName, ipAddr); err != nil {
			logger.Errorf("createNewPost(): store.AddPostToTopic() failed with %s", err)
		}
		http.Redirect(w, r, fmt.Sprintf("/%s/topic?id=%d", model.ForumUrl, topic.Id), 302)
	}
}

// url: /{forum}/newpost[?topicId={topicId}]
func handleNewPost(w http.ResponseWriter, r *http.Request) {
	var err error
	forum := mustGetForum(w, r)
	if forum == nil {
		return
	}

	topicId := 0
	var topic *Topic
	topicIdStr := strings.TrimSpace(r.FormValue("topicId"))
	if topicIdStr != "" {
		if topicId, err = strconv.Atoi(topicIdStr); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
			return
		}
		if topic = forum.Store.TopicById(topicId); topic == nil {
			logger.Noticef("handleNewPost(): invalid topicId: %d\n", topicId)
			http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
			return
		}
	}
	isAdmin := userIsAdmin(forum, getSecureCookie(r))
	sidebar := DoSidebarTemplate(forum, isAdmin)

	//fmt.Printf("handleNewPost(): forum: '%s', topicId: %d\n", forum.ForumUrl, topicId)
	cookie := getSecureCookie(r)
	model := &ModelNewPost{
		Forum:           *forum,
		SidebarHtml:     template.HTML(sidebar),
		AnalyticsCode:   config.AnalyticsCode,
		Num1:            rand.Intn(9) + 1,
		Num2:            rand.Intn(9) + 1,
		TopicId:         topicId,
		LogInOut:        getLogInOut(r, getSecureCookie(r)),
		TwitterUserName: cookie.TwitterUser,
		PrevName:        cookie.AnonUser,
	}
	model.Num3 = model.Num1 + model.Num2

	if r.Method == "POST" {
		createNewPost(w, r, model, topic)
		return
	}

	if topicId != 0 {
		model.PrevSubject = topic.Subject
	}

	ExecTemplate(w, tmplNewPost, model)
}
