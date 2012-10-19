// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// url: /{forum}/postsBy?[user=${userNameInternal}][ip=${ipInternal}]
func handlePostsBy(w http.ResponseWriter, r *http.Request) {
	forum := mustGetForum(w, r)
	if forum == nil {
		return
	}

	var posts []*Post
	userInternal := strings.TrimSpace(r.FormValue("user"))
	ipAddrInternal := strings.TrimSpace(r.FormValue("ip"))
	if userInternal == "" && ipAddrInternal == "" {
		logger.Noticef("handlePostsBy(): missing both user and ip")
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}

	if userInternal != "" {
		posts = forum.Store.GetPostsByUserInternal(userInternal, 50)
	} else {
		posts = forum.Store.GetPostsByIpInternal(ipAddrInternal, 50)
	}

	isAdmin := userIsAdmin(forum, getSecureCookie(r))
	displayPosts := make([]*PostDisplay, 0)
	for _, p := range posts {
		pd := NewPostDisplay(p, forum, isAdmin)
		if pd != nil {
			displayPosts = append(displayPosts, pd)
		}
	}

	model := struct {
		Forum
		SidebarHtml   template.HTML
		Posts         []*PostDisplay
		IsAdmin       bool
		AnalyticsCode *string
		LogInOut      template.HTML
	}{
		Forum:         *forum,
		SidebarHtml:   template.HTML(forum.Sidebar),
		Posts:         displayPosts,
		IsAdmin:       isAdmin,
		AnalyticsCode: config.AnalyticsCode,
		LogInOut:      getLogInOut(r, getSecureCookie(r)),
	}
	ExecTemplate(w, tmplPosts, model)
}
