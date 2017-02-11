// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

// TODO: add an option to log to a file in the format:
// $time E: $msg
// $time N: $msg
// E: is for errors, N: is for notices
// format of $time is TBD (human readable is long, unix timestamp is short
// but not human-readable)

// TODO: gather all errors and email them periodically (e.g. every day) to myself

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kjk/u"
)

// TimestampedMsg is a messsage with a timestamp
type TimestampedMsg struct {
	Time time.Time
	Msg  string
}

// CircularMessagesBuf is a circular buffer for messages
type CircularMessagesBuf struct {
	Msgs []TimestampedMsg
	pos  int
	full bool
}

// TimeStr formats a log timestamp
func (m *TimestampedMsg) TimeStr() string {
	return m.Time.Format("2006-01-02 15:04:05")
}

// TimeSinceStr returns formatted time since log timestamp
func (m *TimestampedMsg) TimeSinceStr() string {
	return u.TimeSinceNowAsString(m.Time)
}

// NewCircularMessagesBuf creates a new circular buffer
func NewCircularMessagesBuf(cap int) *CircularMessagesBuf {
	return &CircularMessagesBuf{
		Msgs: make([]TimestampedMsg, cap, cap),
		pos:  0,
		full: false,
	}
}

// Add adds a message
func (b *CircularMessagesBuf) Add(s string) {
	var msg = TimestampedMsg{time.Now(), s}
	if b.pos == cap(b.Msgs) {
		b.pos = 0
		b.full = true
	}
	b.Msgs[b.pos] = msg
	b.pos++
}

// GetOrdered returns ordered messages
func (b *CircularMessagesBuf) GetOrdered() []*TimestampedMsg {
	size := b.pos
	if b.full {
		size = cap(b.Msgs)
	}
	res := make([]*TimestampedMsg, size, size)
	for i := 0; i < size; i++ {
		p := b.pos - 1 - i
		if p < 0 {
			p = cap(b.Msgs) + p
		}
		res[i] = &b.Msgs[p]
	}
	return res
}

// ServerLogger describes a logger
type ServerLogger struct {
	Errors    *CircularMessagesBuf
	Notices   *CircularMessagesBuf
	UseStdout bool
}

// NewServerLogger creates a logger
func NewServerLogger(errorsMax, noticesMax int, useStdout bool) *ServerLogger {
	l := &ServerLogger{
		Errors:    NewCircularMessagesBuf(errorsMax),
		Notices:   NewCircularMessagesBuf(noticesMax),
		UseStdout: useStdout,
	}
	return l
}

// Error logs an error
func (l *ServerLogger) Error(s string) {
	l.Errors.Add(s)
	fmt.Printf("Error: %s\n", s)
}

// Errorf logs an error
func (l *ServerLogger) Errorf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Errors.Add(s)
	fmt.Printf("Error: %s\n", s)
}

// Notice logs a notice
func (l *ServerLogger) Notice(s string) {
	l.Notices.Add(s)
	fmt.Printf("%s\n", s)
}

// Noticef logs a notice
func (l *ServerLogger) Noticef(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Notices.Add(s)
	fmt.Printf("%s\n", s)
}

// GetErrors returns error messages
func (l *ServerLogger) GetErrors() []*TimestampedMsg {
	return l.Errors.GetOrdered()
}

// GetNotices returns notice messages
func (l *ServerLogger) GetNotices() []*TimestampedMsg {
	return l.Notices.GetOrdered()
}

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
		model.Header.Add("RealIp", getIPAddress(r))
	}

	ExecTemplate(w, tmplLogs, model)
}
