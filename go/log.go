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
	"time"

	"github.com/kjk/u"
)

type TimestampedMsg struct {
	Time time.Time
	Msg  string
}

type CircularMessagesBuf struct {
	Msgs []TimestampedMsg
	pos  int
	full bool
}

func (m *TimestampedMsg) TimeStr() string {
	return m.Time.Format("2006-01-02 15:04:05")
}

func (m *TimestampedMsg) TimeSinceStr() string {
	return u.TimeSinceNowAsString(m.Time)
}

func NewCircularMessagesBuf(cap int) *CircularMessagesBuf {
	return &CircularMessagesBuf{
		Msgs: make([]TimestampedMsg, cap, cap),
		pos:  0,
		full: false,
	}
}

func (b *CircularMessagesBuf) Add(s string) {
	var msg = TimestampedMsg{time.Now(), s}
	if b.pos == cap(b.Msgs) {
		b.pos = 0
		b.full = true
	}
	b.Msgs[b.pos] = msg
	b.pos += 1
}

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

type ServerLogger struct {
	Errors    *CircularMessagesBuf
	Notices   *CircularMessagesBuf
	UseStdout bool
}

func NewServerLogger(errorsMax, noticesMax int, useStdout bool) *ServerLogger {
	l := &ServerLogger{
		Errors:    NewCircularMessagesBuf(errorsMax),
		Notices:   NewCircularMessagesBuf(noticesMax),
		UseStdout: useStdout,
	}
	return l
}

func (l *ServerLogger) Error(s string) {
	l.Errors.Add(s)
	fmt.Printf("Error: %s\n", s)
}

func (l *ServerLogger) Errorf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Errors.Add(s)
	fmt.Printf("Error: %s\n", s)
}

func (l *ServerLogger) Notice(s string) {
	l.Notices.Add(s)
	fmt.Printf("%s\n", s)
}

func (l *ServerLogger) Noticef(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Notices.Add(s)
	fmt.Printf("%s\n", s)
}

func (l *ServerLogger) GetErrors() []*TimestampedMsg {
	return l.Errors.GetOrdered()
}

func (l *ServerLogger) GetNotices() []*TimestampedMsg {
	return l.Notices.GetOrdered()
}
