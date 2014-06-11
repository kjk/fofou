package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kjk/u"
)

var dataDir = ""
var srcDataDir = filepath.Join("..", "appengine", "imported_data")

var APP_NAME = "SumatraPDF"

// data dir is ../../../data on the server or ../../fofoudata locally
// the important part is that it's outside of the code
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "fofoudata")
	if u.PathExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "..", "data")
	if u.PathExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../../../data or ../../fofoudata) doesn't exist")
	return ""
}

func forumDataDir() string {
	return filepath.Join(getDataDir(), "forum")
}

func dataFilePath(app string) string {
	fileName := fmt.Sprintf("%s.txt", app)
	return filepath.Join(forumDataDir(), fileName)
}

type Post struct {
	ForumId        int
	TopicId        int
	OrigTopicId    int
	CreatedOn      time.Time
	MessageSha1    [20]byte
	MessageSha1Str string
	IsDeleted      bool
	IP             string
	UserName       string
	UserEmail      string
	UserHomepage   string

	Id int
}

type Topic struct {
	ForumId   int
	Id        int
	Subject   string
	CreatedOn time.Time
	CreatedBy string
	IsDeleted bool
	Posts     []*Post
}

var newlines = []byte{'\n', '\n'}
var newline = []byte{'\n'}

// "2006-06-05 17:06:34"
func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		log.Fatalf("failed to parse date %s, err: %s", s, err)
	}
	return t
}

func parseTopic(d []byte) *Topic {
	parts := bytes.Split(d, newline)
	topic := &Topic{}
	for _, p := range parts {
		lp := bytes.SplitN(p, []byte{':', ' '}, 2)
		name := string(lp[0])
		val := string(lp[1])
		if "I" == name {
			idparts := strings.Split(val, ".")
			topic.ForumId, _ = strconv.Atoi(idparts[0])
			topic.Id, _ = strconv.Atoi(idparts[1])
		} else if "S" == name {
			topic.Subject = val
		} else if "On" == name {
			topic.CreatedOn = parseTime(val)
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

func parsePost(d []byte) *Post {
	parts := bytes.Split(d, newline)
	post := &Post{}
	for _, p := range parts {
		lp := bytes.SplitN(p, []byte{':', ' '}, 2)
		name := string(lp[0])
		val := string(lp[1])
		if "T" == name {
			idparts := strings.Split(val, ".")
			post.ForumId, _ = strconv.Atoi(idparts[0])
			post.TopicId, _ = strconv.Atoi(idparts[1])
			post.OrigTopicId = post.TopicId
		} else if "On" == name {
			post.CreatedOn = parseTime(val)
		} else if "M" == name {
			post.MessageSha1Str = val
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

func parseTopics(d []byte) []*Topic {
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
	return topics
}

func loadTopics() []*Topic {
	data_dir := filepath.Join("..", "appengine", "imported_data")
	filePath := filepath.Join(data_dir, "topics.txt")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("ReadAll() failed with error %s", err)
	}
	return parseTopics(data)
}

func parsePosts(d []byte) []*Post {
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
	return posts
}

func loadPosts() []*Post {
	filePath := filepath.Join(srcDataDir, "posts.txt")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("ReadAll() failed with error %s", err)
	}
	return parsePosts(data)
}

// for sorting by time
type PostsSeq []*Post

func (s PostsSeq) Len() int      { return len(s) }
func (s PostsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByTime struct{ PostsSeq }

func (s ByTime) Less(i, j int) bool {
	return s.PostsSeq[i].CreatedOn.UnixNano() < s.PostsSeq[j].CreatedOn.UnixNano()
}

func renumberPostIds(topics []*Topic, posts []*Post) []*Topic {
	res := make([]*Topic, 0)
	topicIdToTopic := make(map[int]*Topic)
	deletedTopics := make(map[int]*Topic)
	droppedTopics := 0

	for _, t := range topics {
		if t.ForumId != 1 {
			droppedTopics += 1
			continue
		}
		if t.IsDeleted {
			droppedTopics += 1
			deletedTopics[t.Id] = t
			continue
		}
		topicIdToTopic[t.Id] = t
		t.Posts = make([]*Post, 0)
		res = append(res, t)
	}

	prevId := 0
	for _, t := range topics {
		if t.Id <= prevId {
			panic("Invalid id")
		}
	}

	droppedPosts := 0
	nPosts := 0
	for _, p := range posts {
		if p.ForumId != 1 {
			droppedPosts += 1
			continue
		}

		t, ok := topicIdToTopic[p.TopicId]
		//t := findTopicById(res, p.TopicId)
		if !ok {
			if _, ok = deletedTopics[p.TopicId]; !ok {
				panic("didn't find topic")
			}
			droppedPosts += 1
			continue
		}
		if p.IsDeleted {
			droppedPosts += 1
			continue
		}

		t.Posts = append(t.Posts, p)
		nPosts += 1
	}

	emptyTopics := 0
	res2 := make([]*Topic, 0)
	for _, t := range res {
		if 0 == len(t.Posts) {
			emptyTopics += 1
			continue
		}
		sort.Sort(ByTime{t.Posts})

		p := t.Posts[0]
		if t.CreatedBy != p.UserName {
			fmt.Printf("%v\n", t)
			fmt.Printf("%v\n", p)
			log.Fatalf("Mismatched names: t.CreatedBy=%s != p.UserName=%v", t.CreatedBy, p.UserName)
		}

		for idx, p := range t.Posts {
			p.TopicId = t.Id
			p.Id = idx + 1
		}
		res2 = append(res2, t)
	}
	fmt.Printf("Dropped topics: %d, emptyTopics: %d, dropped posts: %d, total posts: %d\n", droppedTopics, emptyTopics, droppedPosts, nPosts)
	return res2
}

func remSep(s string) string {
	return strings.Replace(s, "|", "", -1)
}

func serTopic(t *Topic) string {
	if t.IsDeleted {
		panic("t.IsDeleted is true")
	}
	return fmt.Sprintf("T%d|%s\n", t.Id, remSep(t.Subject))
}

func ip2str(s string) uint32 {
	var nums [4]uint32
	parts := strings.Split(s, ".")
	for n, p := range parts {
		num, _ := strconv.Atoi(p)
		nums[n] = uint32(num)
	}
	return (nums[0] << 24) | (nums[1] << 16) + (nums[2] << 8) | nums[3]
}

func serPost(p *Post) string {
	if p.IsDeleted {
		panic("p.IsDeleted is true")
	}
	s1 := fmt.Sprintf("%d", p.CreatedOn.Unix())
	s2 := base64.StdEncoding.EncodeToString(p.MessageSha1[:])
	s2 = s2[:len(s2)-1]
	s3 := remSep(p.UserName)
	sIp := fmt.Sprintf("%x", ip2str(p.IP))
	//s4 := remSep(p.UserEmail)
	//s5 := remSep(p.UserHomepage)
	//return fmt.Sprintf("P:%d|%d|%s|%s|%s|%s|%s|%s\n", p.TopicId, p.Id, s1, s2, p.IP, s3, s4, s5)
	return fmt.Sprintf("P%d|%d|%s|%s|%s|%s\n", p.TopicId, p.Id, s1, s2, sIp, s3)
}

func serializePostsAndTopics(topics []*Topic) []string {
	res := make([]string, 0, len(topics)*6)
	for _, t := range topics {
		res = append(res, serTopic(t))
		for _, p := range t.Posts {
			res = append(res, serPost(p))
		}
	}
	return res
}

func blobPath(dir, sha1 string) string {
	d1 := sha1[:2]
	d2 := sha1[2:4]
	return filepath.Join(dir, "blobs", d1, d2, sha1)
}

func copyBlobs(topics []*Topic) error {
	blobsDir := getDataDir()
	for _, t := range topics {
		for _, p := range t.Posts {
			sha1 := p.MessageSha1Str
			srcPath := blobPath(srcDataDir, sha1)
			dstPath := blobPath(blobsDir, sha1)
			if !u.PathExists(srcPath) {
				panic("srcPath doesn't exist")
			}
			if !u.PathExists(dstPath) {
				if err := u.CreateDirIfNotExists(filepath.Dir(dstPath)); err != nil {
					panic("failed to create dir for dstPath")
				}
				if err := u.CopyFile(dstPath, srcPath); err != nil {
					fmt.Printf("CopyFile(%q, %q) failed with %s", dstPath, srcPath, err)
					return err
				}
				fmt.Sprintf("%s=>%s\n", srcPath, dstPath)
			}
		}
	}
	return nil
}

func main() {
	if err := u.CreateDirIfNotExists(forumDataDir()); err != nil {
		panic("failed to create dataDir")
	}

	topics := loadTopics()
	posts := loadPosts()
	topics = renumberPostIds(topics, posts)
	strs := serializePostsAndTopics(topics)

	f, err := os.Create(dataFilePath(APP_NAME))
	if err != nil {
		log.Fatalf("os.Create() failed with %s", err)
	}
	defer f.Close()
	for _, s := range strs {
		_, err = f.WriteString(s)
		if err != nil {
			log.Fatalf("WriteFile() failed with %s", err)
		}
	}
	if err = copyBlobs(topics); err != nil {
		panic("copyBlobs() failed")
	}
}
