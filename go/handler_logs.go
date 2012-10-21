// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"net/http"
)

// TODO: more compact date printing, e.g.:
// "2012-10-03 13:15:31"
// or even group by day, and say:
// 2012-10-03:
//   13:15:31

// url: /logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	cookie := getSecureCookie(r)
	isAdmin := cookie.TwitterUser == "kjk" // only I can see the logs
	model := struct {
		UserIsAdmin bool
		Errors      []*TimestampedMsg
		Notices     []*TimestampedMsg
		Header      *http.Header
	}{
		UserIsAdmin: isAdmin,
	}

	if model.UserIsAdmin {
		model.Errors = logger.GetErrors()
		model.Notices = logger.GetNotices()
	}

	if r.FormValue("show") != "" {
		model.Header = &r.Header
		model.Header.Add("RealIp", getIpAddress(r))
	}

	ExecTemplate(w, tmplLogs, model)
}
