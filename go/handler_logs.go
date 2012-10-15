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
type ModelLogs struct {
	PageTitle   string
	User        string
	UserIsAdmin bool
	RedirectUrl string
	Errors      []*TimestampedMsg
	Notices     []*TimestampedMsg
}

// url: /logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	user := decodeUserFromCookie(r)
	model := &ModelLogs{
		User:        user,
		UserIsAdmin: user == "kjk", // only I can see the logs
		RedirectUrl: r.URL.String(),
	}
	if model.UserIsAdmin {
		model.Errors = logger.GetErrors()
		model.Notices = logger.GetNotices()
	}

	if err := GetTemplates().ExecuteTemplate(w, tmplLogs, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
