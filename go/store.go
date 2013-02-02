// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Post struct {
	Id               int
	CreatedOn        time.Time
	MessageSha1      [20]byte
	UserNameInternal string
	IpAddrInternal   string
	IsDeleted        bool
	Topic            *Topic // for convenience, we link to the topic this post belongs to
}

func (p *Post) IpAddress() string {
	return ipAddrInternalToOriginal(p.IpAddrInternal)
}

func (p *Post) IsTwitterUser() bool {
	return strings.HasPrefix(p.UserNameInternal, "t:")
}

func (p *Post) UserName() string {
	s := p.UserNameInternal
	// note: a hack just for myself
	if s == "t:kjk" {
		return "Krzysztof Kowalczyk"
	}
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

	// those are in the "internal" (more compact) form
	blockedIpAddresses []string
	dataFile           *os.File
}

func stringIndex(arr []string, el string) int {
	for i, s := range arr {
		if s == el {
			return i
		}
	}
	return -1
}

func deleteStringAt(arr *[]string, i int) {
	a := *arr
	l := len(a) - 1
	a[i] = a[l]
	*arr = a[:l]
}

func deleteStringIn(a *[]string, el string) {
	i := stringIndex(*a, el)
	if -1 != i {
		deleteStringAt(a, i)
	}
}

func (t *Topic) IsDeleted() bool {
	for _, p := range t.Posts {
		if !p.IsDeleted {
			return false
		}
	}
	return true
}

// parse: 
// D|1234|1
func parseDelUndel(d []byte) (int, int) {
	s := string(d[1:])
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

// parse:
// T$id|$subject
func parseTopic(line []byte) Topic {
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
	return t
}

func intStrToBool(s string) bool {
	if i, err := strconv.Atoi(s); err == nil {
		if i == 0 {
			return false
		}
		if i == 1 {
			return true
		}
		panic("i is not 0 or 1")
	}
	panic("s is not an integer")
}

// parse:
// B$ipAddr|$isBlocked
func parseBlockUnblockIpAddr(line []byte) (string, bool) {
	s := string(line[1:])
	parts := strings.Split(s, "|")
	if len(parts) != 2 {
		panic("len(parts) != 2")
	}
	return parts[0], intStrToBool(parts[1])
}

// parse:
// P1|1|1148874103|K4hYtOI8xYt5dYH25VQ7Qcbk73A|4b0af66e|Krzysztof Kowalczyk
func parsePost(line []byte, topicIdToTopic map[int]*Topic) Post {
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
	realPostId := len(t.Posts) + 1
	if id != realPostId {
		fmt.Printf("!Unexpected post id:\n")
		fmt.Printf("  %s\n", string(line))
		fmt.Printf("  id: %d, expectedId: %d, topicId: %d\n", topicId, id, realPostId)
		fmt.Printf("  %s\n", t.Subject)
		//TODO: I don't see how this could have happened, but it did, so
		// silently ignore it
		//panic("id != len(t.Posts) + 1")
	}
	post := Post{
		Id:               realPostId,
		CreatedOn:        createdOn,
		UserNameInternal: userName,
		IpAddrInternal:   ipAddrInternal,
		IsDeleted:        false,
		Topic:            t,
	}
	copy(post.MessageSha1[:], msgSha1)
	return post
}

func (store *Store) markIpBlockedOrUnblocked(ipAddrInternal string, blocked bool) {
	if blocked {
		store.blockedIpAddresses = append(store.blockedIpAddresses, ipAddrInternal)
	} else {
		deleteStringIn(&store.blockedIpAddresses, ipAddrInternal)
	}
}

func (store *Store) readExistingData(fileDataPath string) error {
	d, err := ReadFileAll(fileDataPath)
	if err != nil {
		return err
	}

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
		c := line[0]
		// T - topic
		// P - post
		// D - delete post
		// U - undelete post
		// B - block/unblock ipaddr
		switch c {
		case 'T':
			t := parseTopic(line)
			store.topics = append(store.topics, t)
			topicIdToTopic[t.Id] = &store.topics[len(store.topics)-1]
		case 'P':
			post := parsePost(line, topicIdToTopic)
			t := post.Topic
			t.Posts = append(t.Posts, post)
			store.posts = append(store.posts, &t.Posts[len(t.Posts)-1])
		case 'D':
			// D|1234|1
			post := findPostToDelUndel(line, topicIdToTopic)
			if post.IsDeleted {
				panic("post already deleted")
			}
			post.IsDeleted = true
		case 'U':
			// U|1234|1
			post := findPostToDelUndel(line, topicIdToTopic)
			if !post.IsDeleted {
				panic("post already undeleted")
			}
			post.IsDeleted = false
		case 'B':
			// B$ipAddr|$isBlocked
			ipAddr, blocked := parseBlockUnblockIpAddr(line[1:])
			store.markIpBlockedOrUnblocked(ipAddr, blocked)
		default:
			panic("Unexpected line type")
		}
	}
	return nil
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
		topics:    make([]Topic, 0),
	}
	var err error
	if PathExists(dataFilePath) {
		if err = store.readExistingData(dataFilePath); err != nil {
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
	}

	verifyTopics(store.topics)

	store.dataFile, err = os.OpenFile(dataFilePath, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("NewStore(): os.OpenFile(%s) failed with %s", dataFilePath, err.Error())
		return nil, err
	}
	return store, nil
}

func (store *Store) PostsCount() int {
	store.Lock()
	defer store.Unlock()
	n := 0
	for _, topic := range store.topics {
		n += len(topic.Posts)
	}
	return n
}

func (store *Store) TopicsCount() int {
	store.Lock()
	defer store.Unlock()
	return len(store.topics)
}

func (store *Store) GetTopics(nMax, from int, withDeleted bool) ([]*Topic, int) {
	res := make([]*Topic, 0, nMax)
	store.Lock()
	defer store.Unlock()
	n := nMax
	i := from
	for n > 0 {
		idx := len(store.topics) - 1 - i
		if idx < 0 {
			break
		}
		t := &store.topics[idx]
		res = append(res, t)
		n -= 1
		i += 1
	}

	newFrom := i
	if len(store.topics)-1-newFrom <= 0 {
		newFrom = 0
	}
	return res, newFrom
}

// note: could probably speed up with binary search, but given our sizes, we're
// fast enough
func (store *Store) topicByIdUnlocked(id int) *Topic {
	for idx, t := range store.topics {
		if id == t.Id {
			return &store.topics[idx]
		}
	}
	return nil
}

func (store *Store) TopicById(id int) *Topic {
	store.Lock()
	defer store.Unlock()
	return store.topicByIdUnlocked(id)
}

func blobPath(dir, sha1 string) string {
	d1 := sha1[:2]
	d2 := sha1[2:4]
	return filepath.Join(dir, "blobs", d1, d2, sha1)
}

func (store *Store) MessageFilePath(sha1 [20]byte) string {
	sha1Str := hex.EncodeToString(sha1[:])
	return blobPath(store.dataDir, sha1Str)
}

func (store *Store) findPost(topicId, postId int) (*Post, error) {
	topic := store.topicByIdUnlocked(topicId)
	if nil == topic {
		return nil, errors.New("didn't find a topic with this id")
	}
	if postId > len(topic.Posts) {
		return nil, errors.New("didn't find post with this id")
	}

	return &topic.Posts[postId-1], nil
}

func (store *Store) appendString(str string) error {
	_, err := store.dataFile.WriteString(str)
	if err != nil {
		fmt.Printf("appendString() error: %s\n", err.Error())
	}
	return err
}

func (store *Store) DeletePost(topicId, postId int) error {
	store.Lock()
	defer store.Unlock()

	post, err := store.findPost(topicId, postId)
	if err != nil {
		return err
	}
	if post.IsDeleted {
		return errors.New("post already deleted")
	}
	str := fmt.Sprintf("D%d|%d\n", topicId, postId)
	if err = store.appendString(str); err != nil {
		return err
	}
	post.IsDeleted = true
	return nil
}

func (store *Store) UndeletePost(topicId, postId int) error {
	store.Lock()
	defer store.Unlock()

	post, err := store.findPost(topicId, postId)
	if err != nil {
		return err
	}
	if !post.IsDeleted {
		return errors.New("post already not deleted")
	}
	str := fmt.Sprintf("U%d|%d\n", topicId, postId)
	if err = store.appendString(str); err != nil {
		return err
	}
	post.IsDeleted = false
	return nil
}

func ipAddrToInternal(ipAddr string) string {
	var nums [4]byte
	parts := strings.Split(ipAddr, ".")
	if len(parts) == 4 {
		for n, p := range parts {
			num, _ := strconv.Atoi(p)
			nums[n] = byte(num)
		}
		s := hex.EncodeToString(nums[:])
		// note: this is for backwards compatibility to match past
		// behavior when we used to trim leading 0
		if s[0] == '0' {
			s = s[1:]
		}
		return s
	}
	// I assume it's ipv6
	return ipAddr
}

func ipAddrInternalToOriginal(s string) string {
	// an earlier version of ipAddrToInternal would sometimes generate
	// 7-byte string (due to Printf() %x not printing the most significant
	// byte as 0 padded), so we're fixing it up here
	if len(s) == 7 {
		// check if ipv4 in hex form
		s2 := "0" + s
		if d, err := hex.DecodeString(s2); err == nil {
			return fmt.Sprintf("%d.%d.%d.%d", d[0], d[1], d[2], d[3])
		}
	}
	if len(s) == 8 {
		// check if ipv4 in hex form
		if d, err := hex.DecodeString(s); err == nil {
			return fmt.Sprintf("%d.%d.%d.%d", d[0], d[1], d[2], d[3])
		}
	}
	// other format (ipv6?)
	return s
}

func remSep(s string) string {
	return strings.Replace(s, "|", "", -1)
}

func (store *Store) writeMessageAsSha1(msg []byte, sha1 [20]byte) error {
	path := store.MessageFilePath(sha1)
	err := WriteBytesToFile(msg, path)
	if err != nil {
		logger.Errorf("Store.writeMessageAsSha1(): failed to write %s with error %s", path, err.Error())
	}
	return err
}

func (store *Store) blockIp(ipAddr string) {
	s := fmt.Sprintf("B%s|1\n", ipAddrToInternal(ipAddr))
	if err := store.appendString(s); err == nil {
		store.markIpBlockedOrUnblocked(ipAddr, true)
	}
}

func (store *Store) unblockIp(ipAddr string) {
	s := fmt.Sprintf("B%s|0\n", ipAddrToInternal(ipAddr))
	if err := store.appendString(s); err == nil {
		store.markIpBlockedOrUnblocked(ipAddr, false)
	}
}

func (store *Store) addNewPost(msg, user, ipAddr string, topic *Topic, newTopic bool) error {
	msgBytes := []byte(msg)
	sha1 := Sha1OfBytes(msgBytes)
	p := &Post{
		Id:               len(topic.Posts) + 1,
		CreatedOn:        time.Now(),
		UserNameInternal: remSep(user),
		IpAddrInternal:   remSep(ipAddrToInternal(ipAddr)),
		IsDeleted:        false,
		Topic:            topic,
	}
	copy(p.MessageSha1[:], sha1)
	if err := store.writeMessageAsSha1(msgBytes, p.MessageSha1); err != nil {
		return err
	}

	topicStr := ""
	if newTopic {
		topicStr = fmt.Sprintf("T%d|%s\n", topic.Id, topic.Subject)
	}

	s1 := fmt.Sprintf("%d", p.CreatedOn.Unix())
	s2 := base64.StdEncoding.EncodeToString(p.MessageSha1[:])
	s2 = s2[:len(s2)-1] // remove unnecessary '=' from the end
	s3 := p.UserNameInternal
	sIp := p.IpAddrInternal
	postStr := fmt.Sprintf("P%d|%d|%s|%s|%s|%s\n", topic.Id, p.Id, s1, s2, sIp, s3)
	str := topicStr + postStr
	if err := store.appendString(str); err != nil {
		return err
	}
	topic.Posts = append(topic.Posts, *p)
	if newTopic {
		store.topics = append(store.topics, *topic)
	}
	store.posts = append(store.posts, &topic.Posts[len(topic.Posts)-1])
	return nil
}

func (store *Store) CreateNewPost(subject, msg, user, ipAddr string) (int, error) {
	store.Lock()
	defer store.Unlock()

	topic := &Topic{
		Id:      1,
		Subject: remSep(subject),
		Posts:   make([]Post, 0),
	}
	if len(store.topics) > 0 {
		// Id of the last topic + 1
		topic.Id = store.topics[len(store.topics)-1].Id + 1
	}
	err := store.addNewPost(msg, user, ipAddr, topic, true)
	return topic.Id, err
}

func (store *Store) AddPostToTopic(topicId int, msg, user, ipAddr string) error {
	store.Lock()
	defer store.Unlock()

	topic := store.topicByIdUnlocked(topicId)
	if topic == nil {
		return errors.New("invalid topicId")
	}
	return store.addNewPost(msg, user, ipAddr, topic, false)
}

func (store *Store) BlockIp(ipAddrInternal string) {
	store.Lock()
	defer store.Unlock()
	store.blockIp(ipAddrInternal)
}

func (store *Store) UnblockIp(ipAddrInternal string) {
	store.Lock()
	defer store.Unlock()
	store.unblockIp(ipAddrInternal)
}

func (store *Store) IsIpBlocked(ipAddrInternal string) bool {
	store.Lock()
	defer store.Unlock()
	i := stringIndex(store.blockedIpAddresses, ipAddrInternal)
	return i != -1
}

func (store *Store) GetRecentPosts(max int) []*Post {
	store.Lock()
	defer store.Unlock()

	// return the oldest at the beginning of the returned array
	if max > len(store.posts) {
		max = len(store.posts)
	}

	res := make([]*Post, max, max)
	for i := 0; i < max; i++ {
		res[i] = store.posts[len(store.posts)-1-i]
	}
	return res
}

func (store *Store) GetPostsByUserInternal(userNameInternal string, max int) ([]*Post, int) {
	store.Lock()
	defer store.Unlock()

	res := make([]*Post, 0)
	total := 0
	for i := len(store.posts) - 1; i >= 0; i-- {
		p := store.posts[i]
		if p.UserNameInternal == userNameInternal {
			if total < max {
				res = append(res, p)
			}
			total += 1
		}
	}
	return res, total
}

func (store *Store) GetPostsByIpInternal(ipAddrInternal string, max int) ([]*Post, int) {
	store.Lock()
	defer store.Unlock()

	res := make([]*Post, 0)
	total := 0
	for i := len(store.posts) - 1; i >= 0; i-- {
		p := store.posts[i]
		if p.IpAddrInternal == ipAddrInternal {
			if total < max {
				res = append(res, p)
			}
			total += 1
		}
	}
	return res, total
}
