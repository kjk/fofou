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
	store := forum.Store

	var posts []*Post
	userInternal := strings.TrimSpace(r.FormValue("user"))
	ipAddrInternal := strings.TrimSpace(r.FormValue("ip"))
	if userInternal == "" && ipAddrInternal == "" {
		logger.Noticef("handlePostsBy(): missing both user and ip")
		http.Redirect(w, r, fmt.Sprintf("/%s/", forum.ForumUrl), 302)
		return
	}

	var total int
	if userInternal != "" {
		posts, total = store.GetPostsByUserInternal(userInternal, 50)
	} else {
		posts, total = store.GetPostsByIpInternal(ipAddrInternal, 50)
	}

	var ipAddr string
	ipBlocked := false
	if ipAddrInternal != "" {
		ipAddr = ipAddrInternalToOriginal(ipAddrInternal)
		ipBlocked = store.IsIpBlocked(ipAddrInternal)
	}

	isAdmin := userIsAdmin(forum, getSecureCookie(r))
	displayPosts := make([]*PostDisplay, 0)
	for _, p := range posts {
		pd := NewPostDisplay(p, forum, isAdmin)
		if pd != nil {
			displayPosts = append(displayPosts, pd)
		}
	}

	sidebar := DoSidebarTemplate(forum, isAdmin)

	model := struct {
		Forum
		SidebarHtml    template.HTML
		Posts          []*PostDisplay
		TotalCount     int
		IsAdmin        bool
		Nick           string
		IpAddr         string
		IpAddrInternal string
		IpBlocked      bool
		AnalyticsCode  *string
		LogInOut       template.HTML
	}{
		Forum:          *forum,
		SidebarHtml:    template.HTML(sidebar),
		Posts:          displayPosts,
		TotalCount:     total,
		IsAdmin:        isAdmin,
		Nick:           userInternal,
		IpAddr:         ipAddr,
		IpAddrInternal: ipAddrInternal,
		IpBlocked:      ipBlocked,
		AnalyticsCode:  config.AnalyticsCode,
		LogInOut:       getLogInOut(r, getSecureCookie(r)),
	}
	ExecTemplate(w, tmplPosts, model)
}
