package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kjk/u"
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

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	if !u.PathExists(filePath) {
		logger.Noticef("serveFileFromDir() file %q doesn't exist, referer: %q", fileName, getReferer(r))
	}
	http.ServeFile(w, r, filePath)
}

// url: /s/*
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[len("/s/"):]
	serveFileFromDir(w, r, "static", file)
}

// url: /img/*
func handleStaticImg(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[len("/img/"):]
	serveFileFromDir(w, r, "img", file)
}

// url: /robots.txt
func handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	serveFileFromDir(w, r, "static", "robots.txt")
}

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

func getIpAddr(w http.ResponseWriter, r *http.Request) (*Forum, string) {
	forum := mustGetForum(w, r)
	if forum == nil {
		http.Redirect(w, r, "/", 302)
		return nil, ""
	}
	ipAddr := strings.TrimSpace(r.FormValue("ip"))
	if ipAddr == "" {
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return nil, ""
	}
	return forum, ipAddr
}

// url: /{forum}/blockip?ip=${ip}
func handleBlockIP(w http.ResponseWriter, r *http.Request) {
	if forum, ip := getIpAddr(w, r); forum != nil {
		//fmt.Printf("handleBlockIP(): forum: %q, ip: %s\n", forum.ForumUrl, ip)
		forum.Store.BlockIP(ip)
		http.Redirect(w, r, fmt.Sprintf("/%s/postsby?ip=%s", forum.ForumUrl, ip), 302)
	}
}

// url: /{forum}/unblockip?ip=${ip}
func handleUnblockIP(w http.ResponseWriter, r *http.Request) {
	if forum, ip := getIpAddr(w, r); forum != nil {
		//fmt.Printf("handleUnblockIP(): forum: %q, ip: %s\n", forum.ForumUrl, ip)
		forum.Store.UnblockIp(ip)
		http.Redirect(w, r, fmt.Sprintf("/%s/postsby?ip=%s", forum.ForumUrl, ip), 302)
	}
}

// url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	if !isTopLevelURL(r.URL.Path) {
		http.NotFound(w, r)
		return
	}

	model := struct {
		Forums        *[]*Forum
		AnalyticsCode string
	}{
		Forums:        &appState.Forums,
		AnalyticsCode: *config.AnalyticsCode,
	}
	ExecTemplate(w, tmplMain, model)
}

func initHttpHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/", makeTimingHandler(handleMain))
	r.HandleFunc("/{forum}", makeTimingHandler(handleForum))
	r.HandleFunc("/{forum}/", makeTimingHandler(handleForum))
	r.HandleFunc("/{forum}/rss", makeTimingHandler(handleRss))
	r.HandleFunc("/{forum}/rssall", makeTimingHandler(handleRssAll))
	r.HandleFunc("/{forum}/topic", makeTimingHandler(handleTopic))
	r.HandleFunc("/{forum}/postsby", makeTimingHandler(handlePostsBy))
	r.HandleFunc("/{forum}/postdel", makeTimingHandler(handlePostDelete))
	r.HandleFunc("/{forum}/postundel", makeTimingHandler(handlePostUndelete))
	r.HandleFunc("/{forum}/viewraw", makeTimingHandler(handleViewRaw))
	r.HandleFunc("/{forum}/newpost", makeTimingHandler(handleNewPost))
	r.HandleFunc("/{forum}/blockip", makeTimingHandler(handleBlockIP))
	r.HandleFunc("/{forum}/unblockip", makeTimingHandler(handleUnblockIP))

	http.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/favicon.ico", http.NotFound)
	http.HandleFunc("/robots.txt", handleRobotsTxt)
	http.HandleFunc("/logs", handleLogs)
	http.HandleFunc("/s/", makeTimingHandler(handleStatic))
	http.HandleFunc("/img/", makeTimingHandler(handleStaticImg))

	http.Handle("/", r)
}
