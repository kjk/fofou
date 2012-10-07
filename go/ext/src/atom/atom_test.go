// This code is under BSD license. See license-bsd.txt
package atom

import (
	"testing"
	"time"
)

func TestGen(t *testing.T) {
	pubTime, err := time.Parse(time.RFC3339, "2012-09-11T07:39:41Z")
	if err != nil {
		panic("failed to parse time")
	}
	feed := &Feed{
		Title:   "My little feed +=<>&;._-",
		Link:    "http://blog.kowalczyk.info/fofou/feedrss",
		PubDate: pubTime}
	e := &Entry{
		Title:       "Item 1",
		Link:        "http://blog.kowalczyk.info/item/1.html",
		Description: "Item 1 description <>;",
		PubDate:     pubTime}
	feed.AddEntry(e)

	s, err := feed.GenXml()
	if err != nil {
		t.Fatalf("Feed.GenXml() returned error %s", err.Error())
	}
	if s != `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom"><title>My little feed +=&lt;&gt;&amp;;._-</title><link href="http://blog.kowalczyk.info/fofou/feedrss" rel="alternate"></link><id>http://blog.kowalczyk.info/fofou/feedrss</id><updated>2012-09-11T07:39:41Z</updated><entry><title>Item 1</title><link href="http://blog.kowalczyk.info/item/1.html" rel="alternate"></link><updated>2012-09-11T07:39:41Z</updated><id>tag:blog.kowalczyk.info,2012-09-11:/item/1.html</id><summary type="html">Item 1 description &lt;&gt;;</summary></entry></feed>` {
		t.Errorf("wrong result: %s", s)
	}
}
