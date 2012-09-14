package main

import (
	"fmt"
	"net/http"
	"strings"
)

// handler for url: GET /logout?redirect=$redirect
func handleLogout(w http.ResponseWriter, r *http.Request) {
	redirect := strings.TrimSpace(r.FormValue("redirect"))
	if redirect == "" {
		serveErrorMsg(w, fmt.Sprintf("Missing redirect value for /logout"))
		return
	}
	deleteSecureCookie(w)
	http.Redirect(w, r, redirect, 302)
}
