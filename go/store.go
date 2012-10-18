// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Note: to save memory, we don't store id and topic id, because they are
// implicit (id == index withing topic.Posts array + 1)
type Post struct {
	CreatedOn        time.Time
	MessageSha1      [20]byte
	userNameInternal string
	ipAddrInternal   string
	IsDeleted        bool

	// for convenience, we link to the topic this post belongs to
	Topic *Topic
}

func (p *Post) IpAddress() string {
	return ipAddrInternalToOriginal(p.ipAddrInternal)
}

func (p *Post) IsTwitterUser() bool {
	return strings.HasPrefix(p.userNameInternal, "t:")
}

func (p *Post) UserName() string {
	s := p.userNameInternal
	if p.IsTwitterUser() {
		return s[2:]
	}
	return s
}

// in store, we need to distinguish between anonymous users and those that
// are logged in via twitter, so we prepend "t:" to twitter user names
// Note: in future we might add more login methods by adding more 
// prefixes
func MakeInternalUserName(userName string, twitter bool) string {
	if twitter {
		return "t:" + userName
	}
	// we can't have users pretending to be logged in, so if the name typed
	// by the user has ':' as second character, we remove that prefix so that
	// we can use "*:" prefix to distinguish logged in from not-logged in users
	if len(userName) >= 2 && userName[1] == ':' {
		if len(userName) > 2 {
			return userName[2:]
		}
		return userName[:1]
	}
	return userName
}

type Topic struct {
	Id      int
	Subject string
	Posts   []Post
}

type Store struct {
	sync.Mutex
	dataDir   string
	forumName string
	topics    []Topic

	// for some functions it's convenient to traverse the posts ordered by
	// time, so we keep them ordered here, even though they are already stored
	// as part of Topic in topics
	posts []*Post

	dataFile *os.File
}

func (t *Topic) IsDeleted() bool {
	for _, p := range t.Posts {
		if !p.IsDeleted {
			return false
		}
	}
	return true
}

func parseDelUndel(d []byte) (int, int) {
	s := string(d)
	parts := strings.Split(s, "|")
	if len(parts) != 2 {
		panic("len(parts) != 2")
	}
	topicId, err := strconv.Atoi(parts[0])
	if err != nil {
		panic("invalid topicId")
	}
	postId, err := strconv.Atoi(parts[1])
	if err != nil {
		panic("invalid postId")
	}
	return topicId, postId
}

func findPostToDelUndel(d []byte, topicIdToTopic map[int]*Topic) *Post {
	topicId, postId := parseDelUndel(d)
	topic, ok := topicIdToTopic[topicId]
	if !ok {
		panic("no topic with that id")
	}
	if postId > len(topic.Posts) {
		panic("invalid postId")
	}
	return &topic.Posts[postId-1]
}

func parseTopics(d []byte, recentPosts *[]*Post) []Topic {
	topics := make([]Topic, 0)
	topicIdToTopic := make(map[int]*Topic)
	for len(d) > 0 {
		idx := bytes.IndexByte(d, '\n')
		if -1 == idx {
			// TODO: this could happen if the last record was only
			// partially written. Should I just ignore it?
			panic("idx shouldn't be -1")
		}
		line := d[:idx]
		//fmt.Printf("'%s' len(topics)=%d\n", string(line), len(topics))
		d = d[idx+1:]
		if line[0] == 'T' {
			// parse: "T1|Subject"
			s := string(line[1:])
			parts := strings.Split(s, "|")
			if len(parts) != 2 {
				panic("len(parts) != 2")
			}
			subject := parts[1]
			idStr := parts[0]
			id, err := strconv.Atoi(idStr)
			if err != nil {
				panic("idStr is not a number")
			}
			t := Topic{
				Id:      id,
				Subject: subject,
				Posts:   make([]Post, 0),
			}
			topics = append(topics, t)
			topicIdToTopic[t.Id] = &topics[len(topics)-1]
		} else if line[0] == 'P' {
			// parse:
			// P1|1|1148874103|K4hYtOI8xYt5dYH25VQ7Qcbk73A|4b0af66e|Krzysztof Kowalczyk
			s := string(line[1:])
			parts := strings.Split(s, "|")
			if len(parts) != 6 {
				panic("len(parts) != 6")
			}
			topicIdStr := parts[0]
			idStr := parts[1]
			createdOnSecondsStr := parts[2]
			msgSha1b64 := parts[3] + "="
			ipAddrInternal := parts[4]
			userName := parts[5]

			topicId, err := strconv.Atoi(topicIdStr)
			if err != nil {
				panic("topicIdStr not a number")
			}

			id, err := strconv.Atoi(idStr)
			if err != nil {
				panic("idStr not a number")
			}
			createdOnSeconds, err := strconv.Atoi(createdOnSecondsStr)
			if err != nil {
				panic("createdOnSeconds not a number")
			}
			createdOn := time.Unix(int64(createdOnSeconds), 0)
			msgSha1, err := base64.StdEncoding.DecodeString(msgSha1b64)
			if err != nil {
				panic("msgSha1b64 not valid base64")
			}
			if len(msgSha1) != 20 {
				panic("len(msgSha1) != 20")
			}
			t, ok := topicIdToTopic[topicId]
			if !ok {
				panic("didn't find topic with a given topicId")
			}
			if id != len(t.Posts)+1 {
				fmt.Printf("%s\n", string(line))
				fmt.Printf("topicId=%d, id=%d, len(topic.Posts)=%d\n", topicId, id, len(t.Posts))
				fmt.Printf("%v\n", t)
				panic("id != len(t.Posts) + 1")
			}
			post := Post{
				CreatedOn:        createdOn,
				userNameInternal: userName,
				ipAddrInternal:   ipAddrInternal,
				IsDeleted:        false,
				Topic:            t,
			}
			copy(post.MessageSha1[:], msgSha1)
			t.Posts = append(t.Posts, post)
			*recentPosts = append(*recentPosts, &t.Posts[len(t.Posts)-1])
		} else if line[0] == 'D' {
			// D|1234|1
			post := findPostToDelUndel(line[1:], topicIdToTopic)
			if post.IsDeleted {
				panic("post already deleted")
			}
			post.IsDeleted = true
		} else if line[0] == 'U' {
			// U|1234|1
			post := findPostToDelUndel(line[1:], topicIdToTopic)
			if !post.IsDeleted {
				panic("post already undeleted")
			}
			post.IsDeleted = false
		} else {
			panic("Unexpected line type")
		}
	}
	return topics
}

func readExistingData(fileDataPath string, recentPosts *[]*Post) ([]Topic, error) {
	f, err := os.Open(fileDataPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return parseTopics(data, recentPosts), nil
}

func verifyTopics(topics []Topic) {
	for i, t := range topics {
		if 0 == len(t.Posts) {
			fmt.Printf("topics at idx %d (%v) has no posts!\n", i, t)
		}
	}
}

func NewStore(dataDir, forumName string) (*Store, error) {
	dataFilePath := filepath.Join(dataDir, "forum", forumName+".txt")
	store := &Store{
		dataDir:   dataDir,
		forumName: forumName,
		posts:     make([]*Post, 0),
	}
	var err error
	if PathExists(dataFilePath) {
		store.topics, err = readExistingData(dataFilePath, &store.posts)
		if err != nil {
			fmt.Printf("readExistingData() failed with %s", err.Error())
			return nil, err
		}
	} else {
		f, err := os.Create(dataFilePath)
		if err != nil {
			fmt.Printf("NewStore(): os.Create(%s) failed with %s", dataFilePath, err.Error())
			return nil, err
		}
		f.Close()
		store.topics = make([]Topic, 0)
	}

	verifyTopics(store.topics)

	store.dataFile, err = os.OpenFile(dataFilePath, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("NewStore(): os.OpenFile(%s) failed with %s", dataFilePath, err.Error())
		return nil, err
	}
	return store, nil
}

func (s *Store) TopicsCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.topics)
}

func (s *Store) GetTopics(nMax, from int, withDeleted bool) ([]*Topic, int) {
	res := make([]*Topic, 0, nMax)
	s.Lock()
	defer s.Unlock()
	n := nMax
	i := from
	for n > 0 {
		idx := len(s.topics) - 1 - i
		if idx < 0 {
			break
		}
		t := &s.topics[idx]
		res = append(res, t)
		n -= 1
		i += 1
	}

	newFrom := i
	if len(s.topics)-1-newFrom <= 0 {
		newFrom = 0
	}
	return res, newFrom
}

// note: could probably speed up with binary search, but given our sizes, we're
// fast enough
func (s *Store) topicByIdUnlocked(id int) *Topic {
	for idx, t := range s.topics {
		if id == t.Id {
			return &s.topics[idx]
		}
	}
	return nil
}

func (s *Store) TopicById(id int) *Topic {
	s.Lock()
	defer s.Unlock()
	return s.topicByIdUnlocked(id)
}

func blobPath(dir, sha1 string) string {
	d1 := sha1[:2]
	d2 := sha1[2:4]
	return filepath.Join(dir, "blobs", d1, d2, sha1)
}

func (s *Store) MessageFilePath(sha1 [20]byte) string {
	sha1Str := hex.EncodeToString(sha1[:])
	return blobPath(s.dataDir, sha1Str)
}

func (s *Store) findPost(topicId, postId int) (*Post, error) {
	topic := s.topicByIdUnlocked(topicId)
	if nil == topic {
		return nil, errors.New("didn't find a topic with this id")
	}
	if postId > len(topic.Posts) {
		return nil, errors.New("didn't find post with this id")
	}

	return &topic.Posts[postId-1], nil
}

func (s *Store) appendString(str string) error {
	_, err := s.dataFile.WriteString(str)
	if err != nil {
		fmt.Printf("appendString() error: %s\n", err.Error())
	}
	return err
}

func (s *Store) DeletePost(topicId, postId int) error {
	s.Lock()
	defer s.Unlock()

	post, err := s.findPost(topicId, postId)
	if err != nil {
		return err
	}
	if post.IsDeleted {
		return errors.New("post already deleted")
	}
	str := fmt.Sprintf("D%d|%d\n", topicId, postId)
	if err = s.appendString(str); err != nil {
		return err
	}
	post.IsDeleted = true
	return nil
}

func (s *Store) UndeletePost(topicId, postId int) error {
	s.Lock()
	defer s.Unlock()

	post, err := s.findPost(topicId, postId)
	if err != nil {
		return err
	}
	if !post.IsDeleted {
		return errors.New("post already not deleted")
	}
	str := fmt.Sprintf("U%d|%d\n", topicId, postId)
	if err = s.appendString(str); err != nil {
		return err
	}
	post.IsDeleted = false
	return nil
}

func ipAddrToInternal(ipAddr string) string {
	var nums [4]uint32
	parts := strings.Split(ipAddr, ".")
	if len(parts) == 4 {
		for n, p := range parts {
			num, _ := strconv.Atoi(p)
			nums[n] = uint32(num)
		}
		n := (nums[0] << 24) | (nums[1] << 16) + (nums[2] << 8) | nums[3]
		return fmt.Sprintf("%x", n)
	}
	// I assume it's ipv6
	return ipAddr
}

func ipAddrInternalToOriginal(s string) string {
	// check if ipv4 in hex form
	if len(s) == 8 {
		if d, err := hex.DecodeString(s); err != nil {
			return s
		} else {
			return fmt.Sprintf("%d.%d.%d.%d", d[0], d[1], d[2], d[3])
		}
	}
	// other format (ipv6?)
	return s
}

func remSep(s string) string {
	return strings.Replace(s, "|", "", -1)
}

func (s *Store) writeMessageAsSha1(msg []byte, sha1 [20]byte) error {
	path := s.MessageFilePath(sha1)
	err := WriteBytesToFile(msg, path)
	if err != nil {
		logger.Errorf("Store.writeMessageAsSha1(): failed to write %s with error %s", path, err.Error())
	}
	return err
}

func (s *Store) addNewPost(msg, user, ipAddr string, topic *Topic, newTopic bool) error {
	msgBytes := []byte(msg)
	sha1 := Sha1OfBytes(msgBytes)
	p := &Post{
		CreatedOn:        time.Now(),
		userNameInternal: remSep(user),
		ipAddrInternal:   remSep(ipAddrToInternal(ipAddr)),
		IsDeleted:        false,
		Topic:            topic,
	}
	copy(p.MessageSha1[:], sha1)
	if err := s.writeMessageAsSha1(msgBytes, p.MessageSha1); err != nil {
		return err
	}

	postId := len(topic.Posts) + 1

	topicStr := ""
	if newTopic {
		topicStr = fmt.Sprintf("T%d|%s\n", topic.Id, topic.Subject)
	}

	s1 := fmt.Sprintf("%d", p.CreatedOn.Unix())
	s2 := base64.StdEncoding.EncodeToString(p.MessageSha1[:])
	s2 = s2[:len(s2)-1] // remove unnecessary '=' from the end
	s3 := p.userNameInternal
	sIp := p.ipAddrInternal
	postStr := fmt.Sprintf("P%d|%d|%s|%s|%s|%s\n", topic.Id, postId, s1, s2, sIp, s3)
	str := topicStr + postStr
	if err := s.appendString(str); err != nil {
		return err
	}
	topic.Posts = append(topic.Posts, *p)
	if newTopic {
		s.topics = append(s.topics, *topic)
	}
	s.posts = append(s.posts, &topic.Posts[len(topic.Posts)-1])
	return nil
}

func (s *Store) CreateNewPost(subject, msg, user, ipAddr string) error {
	s.Lock()
	defer s.Unlock()

	topic := &Topic{
		Id:      1,
		Subject: remSep(subject),
		Posts:   make([]Post, 0),
	}
	if len(s.topics) > 0 {
		// Id of the last topic + 1
		topic.Id = s.topics[len(s.topics)-1].Id + 1
	}
	return s.addNewPost(msg, user, ipAddr, topic, true)
}

func (s *Store) AddPostToTopic(topicId int, msg, user, ipAddr string) error {
	s.Lock()
	defer s.Unlock()

	topic := s.topicByIdUnlocked(topicId)
	if topic == nil {
		return errors.New("invalid topicId")
	}
	return s.addNewPost(msg, user, ipAddr, topic, false)
}

func (s *Store) GetRecentPosts() []*Post {
	s.Lock()
	defer s.Unlock()

	first := len(s.posts) - 25 // get 25 last posts
	if first < 0 {
		first = 0
	}
	return s.posts[first:]
}

func (s *Store) GetPostsByUserInternal(userInternal string) []*Post {
	s.Lock()
	defer s.Unlock()

	// TODO: actually filter by user
	first := len(s.posts) - 25 // get 25 last posts
	if first < 0 {
		first = 0
	}
	return s.posts[first:]
}

func (s *Store) GetPostsByIpInternal(ipInternal string) []*Post {
	s.Lock()
	defer s.Unlock()

	// TODO: actually filter by ip
	first := len(s.posts) - 25 // get 25 last posts
	if first < 0 {
		first = 0
	}
	return s.posts[first:]
}
