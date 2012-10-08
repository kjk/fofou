// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"code.google.com/p/gorilla/securecookie"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"oauth"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	configPath   = flag.String("config", "secrets.json", "Path to configuration file")
	httpAddr     = flag.String("addr", ":5010", "HTTP server address")
	logPath      = flag.String("log", "stdout", "where to log")
	inProduction = flag.Bool("production", false, "are we running in production")
	cookieName   = "ckie"
)

var (
	oauthClient = oauth.Client{
		TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
		ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
		TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
	}

	config = struct {
		TwitterOAuthCredentials *oauth.Credentials
		Forums                  []ForumConfig
		CookieAuthKeyHexStr     *string
		CookieEncrKeyHexStr     *string
	}{
		&oauthClient.Credentials,
		nil,
		nil,
		nil,
	}
	logger        *ServerLogger
	cookieAuthKey []byte
	cookieEncrKey []byte
	secureCookie  *securecookie.SecureCookie

	// this is where we store information about users and translation.
	// All in one place because I expect this data to be small
	dataDir string

	staticDir = "static"

	appState = AppState{}

	tmplMain        = "main.html"
	tmplForum       = "forum.html"
	templateNames   = [...]string{tmplMain, tmplForum}
	templatePaths   []string
	templates       *template.Template
	reloadTemplates = true
	alwaysLogTime   = true
)

// a static configuration of a single forum
type ForumConfig struct {
	Title         string
	ForumUrl      string
	WebsiteUrl    string
	Sidebar       string
	Tagline       string
	DataDir       string
	AnalyticsCode string
	// we authenticate only with Twitter, this is the twitter user name
	// of the admin user
	AdminTwitterUser string
}

type User struct {
	Login string
}

type Forum struct {
	ForumConfig
	Topics        []*Topic
	IsDeleted     bool
	Id            int
	CommentsCount int
	MsgShort      string
	Subject       string
	CreatedBy     string
	CreatedOn     time.Time
}

type AppState struct {
	Users  []*User
	Forums []*Forum
}

func NewForum(config *ForumConfig) *Forum {
	forum := &Forum{ForumConfig: *config}
	logger.Noticef("Created %s forum\n", forum.Title)
	return forum
}

// data dir is ../data on the server or ../apptranslatordata locally
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "fofoudata")
	if FileExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "data")
	if FileExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../../data or ../fofoudata) doesn't exist")
	return ""
}

func findForum(forumUrl string) *Forum {
	for _, f := range appState.Forums {
		if f.ForumUrl == forumUrl {
			return f
		}
	}
	return nil
}

func forumAlreadyExists(siteUrl string) bool {
	return nil != findForum(siteUrl)
}

func forumInvalidField(forum *Forum) string {
	forum.Title = strings.TrimSpace(forum.Title)
	if forum.Title == "" {
		return "Title"
	}
	if forum.ForumUrl == "" {
		return "ForumUrl"
	}
	if forum.WebsiteUrl == "" {
		return "WebsiteUrl"
	}
	if forum.DataDir == "" {
		return "DataDir"
	}
	if forum.AdminTwitterUser == "" {
		return "AdminTwitterUser"
	}
	return ""
}

func addForum(forum *Forum) error {
	if invalidField := forumInvalidField(forum); invalidField != "" {
		return errors.New(fmt.Sprintf("Forum has invalid field '%s'", invalidField))
	}
	if forumAlreadyExists(forum.ForumUrl) {
		return errors.New("Forum already exists")
	}

	/*if err := readForumData(app); err != nil {
		return err
	}*/
	appState.Forums = append(appState.Forums, forum)
	return nil
}

func GetTemplates() *template.Template {
	if reloadTemplates || (nil == templates) {
		if 0 == len(templatePaths) {
			for _, name := range templateNames {
				templatePaths = append(templatePaths, filepath.Join("tmpl", name))
			}
		}
		templates = template.Must(template.ParseFiles(templatePaths...))
	}
	return templates
}

func isTopLevelUrl(url string) bool {
	return 0 == len(url) || "/" == url
}

func serve404(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func serveErrorMsg(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func userIsAdmin(f *Forum, user string) bool {
	return user == f.AdminTwitterUser
}

// readSecrets reads the configuration file from the path specified by
// the config command line flag.
func readSecrets(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		return err
	}
	cookieAuthKey, err = hex.DecodeString(*config.CookieAuthKeyHexStr)
	if err != nil {
		return err
	}
	cookieEncrKey, err = hex.DecodeString(*config.CookieEncrKeyHexStr)
	if err != nil {
		return err
	}
	secureCookie = securecookie.New(cookieAuthKey, cookieEncrKey)
	// verify auth/encr keys are correct
	val := map[string]string{
		"foo": "bar",
	}
	_, err = secureCookie.Encode(cookieName, val)
	if err != nil {
		// for convenience, if the auth/encr keys are not set,
		// generate valid, random value for them
		auth := securecookie.GenerateRandomKey(32)
		encr := securecookie.GenerateRandomKey(32)
		fmt.Printf("auth: %s\nencr: %s\n", hex.EncodeToString(auth), hex.EncodeToString(encr))
	}
	// TODO: somehow verify twitter creds
	return err
}

func makeTimingHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		startTime := time.Now()
		fn(w, r)
		duration := time.Now().Sub(startTime)
		// log urls that take long time to generate i.e. over 1 sec in production
		// or over 0.1 sec in dev
		shouldLog := duration.Seconds() > 1.0
		if alwaysLogTime && duration.Seconds() > 0.1 {
			shouldLog = true
		}
		if shouldLog {
			url := r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
			}
			logger.Noticef("'%s' took %f seconds to serve\n", url, duration.Seconds())
		}
	}
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
			post.ForumId, _ = strconv.Atoi(idparts[0])
			post.Id, _ = strconv.Atoi(idparts[1])
		} else if "On" == name {
			// TODO: change to time.Time
			post.CreatedOn = val
		} else if "M" == name {
			sha1, err := hex.DecodeString(val)
			if err != nil || len(sha1) != 20 {
				log.Fatalf("error decoding M")
			}
			copy(post.MessageSha1, sha1)
		} else if "D" == name {
			post.IsDeleted = ("True" == val)
		} else if "IP" == name {
			post.IP = val
		} else if "UN" == name {
			post.UserName = val
		} else if "UE" == name {
			post.CreatedBy = val
		} else if "UH" == name {
			post.CreatedBy = val
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
	err := ioutil.WriteFile("topics_new.txt", []byte(s), 0600)
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
	fmt.Printf("topics: %d, unique names: %d, len(s) = %d\n", len(posts), uniqueNames, len(s))
	err := ioutil.WriteFile("posts_new.txt", []byte(s), 0600)
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
	parsePosts()
}

func main() {
	loadTopics()
	return

	// set number of goroutines to number of cpus, but capped at 4 since
	// I don't expect this to be heavily trafficed website
	ncpu := runtime.NumCPU()
	if ncpu > 4 {
		ncpu = 4
	}
	runtime.GOMAXPROCS(ncpu)
	flag.Parse()

	if *inProduction {
		reloadTemplates = false
		alwaysLogTime = false
	}

	logger = NewServerLogger(256, 256)

	if err := readSecrets(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err.Error())
	}

	for _, forumData := range config.Forums {
		f := NewForum(&forumData)
		if err := addForum(f); err != nil {
			log.Fatalf("Failed to add the forum: %s, err: %s\n", f.Title, err.Error())
		}
	}

	if len(appState.Forums) == 0 {
		log.Fatalf("No forums defined in secrets.json")
	}

	r := mux.NewRouter()
	r.HandleFunc("/", makeTimingHandler(handleMain))
	http.HandleFunc("/s/", makeTimingHandler(handleStatic))

	r.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	r.HandleFunc("/login", handleLogin)
	r.HandleFunc("/logout", handleLogout)

	r.HandleFunc("/{forum}", makeTimingHandler(handleForum))
	r.HandleFunc("/{forum}/rss", makeTimingHandler(handleRss))

	http.Handle("/", r)
	msg := fmt.Sprintf("Started runing on %s", *httpAddr)
	logger.Noticef(msg)
	println(msg)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")
}
