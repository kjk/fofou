package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var dataDir = ""
var APP_NAME = "SumatraPDF"

// data dir is ../../../data on the server or ../../fofoudata locally
// the important part is that it's outside of the code
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "fofoudata")
	if PathExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "..", "data")
	if PathExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../../../data or ../../fofoudata) doesn't exist")
	return ""
}

func dataFilePath(app string) string {
	return filepath.Join(getDataDir(), app, "data.txt")
}

type Topic struct {
	ForumId   int
	Id        int
	Subject   string
	CreatedOn string
	CreatedBy string
	IsDeleted bool
}

var newlines = []byte{'\n', '\n'}
var newline = []byte{'\n'}

func parseTopic(d []byte) *Topic {
	parts := bytes.Split(d, newline)
	topic := &Topic{}
	for _, p := range parts {
		lp := bytes.Split(p, []byte{':', ' '})
		name := string(lp[0])
		val := string(lp[1])
		if "I" == name {
			idparts := strings.Split(val, ".")
			topic.ForumId, _ = strconv.Atoi(idparts[0])
			topic.Id, _ = strconv.Atoi(idparts[1])
		} else if "S" == name {
			topic.Subject = val
		} else if "On" == name {
			// TODO: change to time.Time
			topic.CreatedOn = val
		} else if "By" == name {
			topic.CreatedBy = val
		} else if "D" == name {
			topic.IsDeleted = ("True" == val)
		} else {
			log.Fatalf("Unknown topic name: %s\n", name)
		}
	}
	return topic
}

type Post struct {
	TopicId      int
	Id           int
	CreatedOn    string
	MessageSha1  [20]byte
	IsDeleted    bool
	IP           string
	UserName     string
	UserEmail    string
	UserHomepage string
}

/*
T: 1.2
M: 2b8858b4e23cc58b797581f6e5543b41c6e4ef70
On: 2006-05-29 03:41:43
D: False
IP: 75.10.246.110
UN: Krzysztof Kowalczyk
UE: kkowalczyk@gmail.com
UH: http://blog.kowalczyk.info
*/

func parsePost(d []byte) *Post {
	parts := bytes.Split(d, newline)
	post := &Post{}
	for _, p := range parts {
		lp := bytes.Split(p, []byte{':', ' '})
		name := string(lp[0])
		val := string(lp[1])
		if "T" == name {
			idparts := strings.Split(val, ".")
			post.TopicId, _ = strconv.Atoi(idparts[0])
			post.Id, _ = strconv.Atoi(idparts[1])
		} else if "On" == name {
			// TODO: change to time.Time
			post.CreatedOn = val
		} else if "M" == name {
			sha1, err := hex.DecodeString(val)
			if err != nil || len(sha1) != 20 {
				log.Fatalf("error decoding M")
			}
			copy(post.MessageSha1[:], sha1)
		} else if "D" == name {
			post.IsDeleted = ("True" == val)
		} else if "IP" == name {
			post.IP = val
		} else if "UN" == name {
			post.UserName = val
		} else if "UE" == name {
			post.UserEmail = val
		} else if "UH" == name {
			post.UserHomepage = val
		} else {
			log.Fatalf("Unknown post name: %s\n", name)
		}
	}
	return post
}

/* type Topic struct {
	ForumId   int
	Id        int
	Subject   string
	CreatedOn string
	CreatedBy string
	IsDeleted bool
}*/

var sep = "|"

func dumpTopics(topics []*Topic) (string, int) {
	s := ""
	names := make(map[string]int)
	for _, t := range topics {
		if t.IsDeleted {
			continue
		}
		subject := strings.Replace(t.Subject, sep, "", -1)
		by := strings.Replace(t.CreatedBy, sep, "", -1)
		if n, ok := names[by]; ok {
			names[by] = n + 1
		} else {
			names[by] = 1
		}
		s += fmt.Sprintf("%d.%d|%s|%s|%s\n", t.ForumId, t.Id, subject, t.CreatedOn, by)
	}
	return s, len(names)
}

func dumpPosts(posts []*Post) (string, int) {
	s := ""
	names := make(map[string]int)
	for _, p := range posts {
		if p.IsDeleted {
			continue
		}
		s += fmt.Sprintf("%d|%d\n", p.TopicId, p.Id)
	}
	return s, len(names)
}

func parseTopics(d []byte) {
	topics := make([]*Topic, 0)
	for len(d) > 0 {
		idx := bytes.Index(d, newlines)
		if idx == -1 {
			break
		}
		topic := parseTopic(d[:idx])
		topics = append(topics, topic)
		d = d[idx+2:]
	}
	s, uniqueNames := dumpTopics(topics)
	fmt.Printf("topics: %d, unique names: %d, len(s) = %d\n", len(topics), uniqueNames, len(s))

	f, err := os.Create(dataFilePath(APP_NAME))
	if err != nil {
		log.Fatalf("os.Create() failed with %s", err.Error())
	}
	defer f.Close()
	_, err = f.WriteString(s)
	if err != nil {
		log.Fatalf("WriteFile() failed with %s", err.Error())
	}
}

func parsePosts(d []byte) {
	posts := make([]*Post, 0)
	for len(d) > 0 {
		idx := bytes.Index(d, newlines)
		if idx == -1 {
			break
		}
		post := parsePost(d[:idx])
		posts = append(posts, post)
		d = d[idx+2:]
	}
	s, uniqueNames := dumpPosts(posts)
	fmt.Printf("posts: %d, unique names: %d, len(s) = %d\n", len(posts), uniqueNames, len(s))

	f, err := os.OpenFile(dataFilePath(APP_NAME), os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("os.Open() failed with %s", err.Error())
	}
	defer f.Close()
	_, err = f.WriteString(s)
	if err != nil {
		log.Fatalf("WriteFile() failed with %s", err.Error())
	}
}

func loadTopics() {
	data_dir := filepath.Join("..", "appengine", "imported_data")
	file_path := filepath.Join(data_dir, "topics.txt")
	f, err := os.Open(file_path)
	if err != nil {
		fmt.Printf("failed to open %s with error %s", file_path, err.Error())
		return
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Printf("ReadAll() failed with error %s", err.Error())
		return
	}
	parseTopics(data)
}

func loadPosts() {
	data_dir := filepath.Join("..", "appengine", "imported_data")
	file_path := filepath.Join(data_dir, "posts.txt")
	f, err := os.Open(file_path)
	if err != nil {
		fmt.Printf("failed to open %s with error %s", file_path, err.Error())
		return
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Printf("ReadAll() failed with error %s", err.Error())
		return
	}
	parsePosts(data)
}

func main() {
	loadTopics()
	loadPosts()
}
