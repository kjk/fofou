// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"fmt"
	"net/http"
	"strings"
)

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
func handleBlockIp(w http.ResponseWriter, r *http.Request) {
	if forum, ip := getIpAddr(w, r); forum != nil {
		fmt.Printf("handleBlockIp(): forum: '%s', ip: %s\n", forum.ForumUrl, ip)
		forum.Store.BlockIp(ip)
		http.Redirect(w, r, fmt.Sprintf("/%s/postsby?ip=%s", forum.ForumUrl, ip), 302)
	}
}

// url: /{forum}/unblockip?ip=${ip}
func handleUnblockIp(w http.ResponseWriter, r *http.Request) {
	if forum, ip := getIpAddr(w, r); forum != nil {
		fmt.Printf("handleUnblockIp(): forum: '%s', ip: %s\n", forum.ForumUrl, ip)
		forum.Store.UnblockIp(ip)
		http.Redirect(w, r, fmt.Sprintf("/%s/postsby?ip=%s", forum.ForumUrl, ip), 302)
	}
}
