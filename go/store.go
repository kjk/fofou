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
	CreatedOn    time.Time
	MessageSha1  [20]byte
	UserName     string
	IpAddressHex string // TODO: string or something else?
	IsDeleted    bool
}

type Topic struct {
	Id      int
	Subject string
	Posts   []Post
}

type Store struct {
	dataDir string
	topics  []Topic

	dataFile *os.File
	mu       sync.Mutex // to serialize writes
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

func parseTopics(d []byte) []Topic {
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
			ipAddrHexStr := parts[4]
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
				CreatedOn:    createdOn,
				UserName:     userName,
				IpAddressHex: ipAddrHexStr,
				IsDeleted:    false,
			}
			copy(post.MessageSha1[:], msgSha1)
			t.Posts = append(t.Posts, post)
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

func readExistingData(fileDataPath string) ([]Topic, error) {
	f, err := os.Open(fileDataPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return parseTopics(data), nil
}

func verifyTopics(topics []Topic) {
	for i, t := range topics {
		if 0 == len(t.Posts) {
			fmt.Printf("topics at idx %d (%v) has no posts!\n", i, t)
		}
	}
}

func NewStore(dataDir string) (*Store, error) {
	dataFilePath := filepath.Join(dataDir, "data.txt")
	store := &Store{dataDir: dataDir}
	var err error
	if PathExists(dataFilePath) {
		store.topics, err = readExistingData(dataFilePath)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.topics)
}

func (s *Store) GetTopics(nMax, from int, withDeleted bool) ([]*Topic, int) {
	res := make([]*Topic, 0, nMax)
	s.mu.Lock()
	defer s.mu.Unlock()
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

/*
func findTopicById(topics []*Topic, id int) *Topic {
	min := 0
	max := len(topics) - 1
	for max >= min {
		mid := min + ((max - min) / 2)
		topicId := topics[mid].Id
		if topicId == id {
			return topics[mid]
		}
		if id > topicId {
			min = mid
		} else {
			max = mid
		}
	}
	return nil
}
*/

func (s *Store) topicByIdUnlocked(id int) *Topic {
	// TODO: binary search?
	for idx, t := range s.topics {
		if id == t.Id {
			return &s.topics[idx]
		}
	}
	return nil
}

func (s *Store) TopicById(id int) *Topic {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.topicByIdUnlocked(id)
}

func blobPath(dir, sha1 string) string {
	d1 := sha1[:2]
	d2 := sha1[2:4]
	return filepath.Join(dir, "..", "blobs", d1, d2, sha1)
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
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

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
