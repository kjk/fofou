package main

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Note: to save memory, we don't store id because it is implicit
// (id == index withing topic.Posts array + 1)
type Post struct {
	TopicId      int
	CreatedOn    time.Time
	MessageSha1  [20]byte
	UserName     string
	IpAddressHex string // TODO: string or something else?
	IsDeleted    bool
}

// Note: to save memory, we don't store id because it is implicit
// (id == index withing topics array + 1)
type Topic struct {
	Subject   string
	Posts     []Post
	IsDeleted bool
}

type Store struct {
	dataDir string
	topics  []Topic

	dataFile *os.File
	mu       sync.Mutex // to serialize writes
}

func parseTopics(d []byte) []Topic {
	topics := make([]Topic, 0)
	for len(d) > 0 {
		idx := bytes.IndexByte(d, '\n')
		if -1 == idx {
			// TODO: this could happen if the last record was only
			// partially written. Should I just ignore it?
			panic("idx shouldn't be -1")
		}
		line := d[:idx]
		d = d[:idx+1]
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
			if len(topics)+1 != id {
				panic("id should be == len(topics)+1")
			}
			t := Topic{
				Subject:   subject,
				IsDeleted: false,
			}
			topics = append(topics, t)
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
			topic := topics[topicId-1]
			if id != len(topic.Posts)+1 {
				panic("id != len(topic.Posts) + 1")
			}
			post := Post{
				TopicId:      topicId,
				CreatedOn:    createdOn,
				UserName:     userName,
				IpAddressHex: ipAddrHexStr,
				IsDeleted:    false,
			}
			copy(post.MessageSha1[:], msgSha1)
			topic.Posts = append(topic.Posts, post)
		} else if line[0] == 'D' {
			// TODO: parse:
			// DT1 or DP1
		} else if line[0] == 'U' {
			// TODO: parse:
			// UT1 or UP1
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

func NewStore(dataDir string) (*Store, error) {
	dataFilePath := filepath.Join(dataDir, "data.txt")
	store := &Store{}
	var err error
	if PathExists(dataFilePath) {
		store.topics, err = readExistingData(dataFilePath)
		if err != nil {
			return nil, err
		}
	} else {
		f, err := os.Create(dataFilePath)
		if err != nil {
			return nil, err
		}
		f.Close()
		store.topics = make([]Topic, 0)
	}
	store.dataFile, err = os.OpenFile(dataFilePath, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) TopicsCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.topics)
}
