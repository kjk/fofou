// This code is under BSD license
// Written by Krzysztof Kowalczyk http://blog.kowalczyk.info
package atom

import (
	"encoding/xml"
	"net/url"
	"time"
)

// Generates Atom feed as XML

const ns = "http://www.w3.org/2005/Atom"

type Feed struct {
	Title   string
	Link    string
	PubDate time.Time
	entries []*Entry
}

type Entry struct {
	Id          string
	Title       string
	Link        string
	ContentHtml string
	PubDate     time.Time
}

func (f *Feed) AddEntry(e *Entry) {
	f.entries = append(f.entries, e)
}

type entryContent struct {
	S    string `xml:",chardata"`
	Type string `xml:"type,attr"`
}

type entryXml struct {
	XMLName xml.Name `xml:"entry"`
	Title   string   `xml:"title"`
	Link    *linkXml
	Updated string        `xml:"updated"`
	Id      string        `xml:"id"`
	Content *entryContent `xml:"content"`
}

type linkXml struct {
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr"`
}

type feedXml struct {
	XMLName xml.Name `xml:"feed"`
	Ns      string   `xml:"xmlns,attr"`
	Title   string   `xml:"title"`
	Link    *linkXml
	Id      string `xml:"id"`
	Updated string `xml:"updated"`
	Entries []*entryXml
}

func newEntryXml(e *Entry) *entryXml {
	s := &entryContent{e.ContentHtml, "html"}
	id := e.Id
	// generate id if not provided
	if id == "" {
		// <id>tag:blog.kowalczyk.info,2012-09-11:/item/1.html</id>
		idDate := e.PubDate.Format("2006-01-02")
		id = "tag:" + e.Link + "," + idDate + ":/invalid.html"
		if url, err := url.Parse(e.Link); err == nil {
			id = "tag:" + url.Host + "," + idDate + ":" + url.Path
		}
	}
	x := &entryXml{
		Title:   e.Title,
		Link:    &linkXml{Href: e.Link, Rel: "alternate"},
		Content: s,
		Id:      id,
		Updated: e.PubDate.Format(time.RFC3339)}
	return x
}

func (f *Feed) GenXmlCompact() (string, error) {
	feed := &feedXml{
		Ns:      ns,
		Title:   f.Title,
		Link:    &linkXml{Href: f.Link, Rel: "alternate"},
		Id:      f.Link,
		Updated: f.PubDate.Format(time.RFC3339),
	}
	for _, e := range f.entries {
		feed.Entries = append(feed.Entries, newEntryXml(e))
	}
	data, err := xml.Marshal(feed)
	if err != nil {
		return "", err
	}
	s := xml.Header[:len(xml.Header)-1] + string(data)
	return s, nil
}

func (f *Feed) GenXml() (string, error) {
	feed := &feedXml{
		Ns:      ns,
		Title:   f.Title,
		Link:    &linkXml{Href: f.Link, Rel: "alternate"},
		Id:      f.Link,
		Updated: f.PubDate.Format(time.RFC3339),
	}
	for _, e := range f.entries {
		feed.Entries = append(feed.Entries, newEntryXml(e))
	}
	data, err := xml.MarshalIndent(feed, " ", " ")
	if err != nil {
		return "", err
	}
	s := xml.Header[:len(xml.Header)-1] + string(data)
	return s, nil
}
