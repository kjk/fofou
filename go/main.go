// This code is in Public Domain. Take all the code you want, we'll just write more.
package main

import (
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
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	configPath   = flag.String("config", "config.json", "Path to configuration file")
	httpAddr     = flag.String("addr", ":5010", "HTTP server address")
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

	appState = AppState{
		Users:  make([]*User, 0),
		Forums: make([]*Forum, 0),
	}

	tmplMain        = "main.html"
	tmplForum       = "forum.html"
	tmplTopic       = "topic.html"
	templateNames   = [...]string{tmplMain, tmplForum, tmplTopic}
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
	Store *Store
}

type AppState struct {
	Users  []*User
	Forums []*Forum
}

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

func forumDataDir(forumDir string) string {
	return filepath.Join(getDataDir(), forumDir)
}

func NewForum(config *ForumConfig) *Forum {
	forum := &Forum{ForumConfig: *config}
	store, err := NewStore(forumDataDir(config.DataDir))
	if err != nil {
		panic("failed to create store for a forum")
	}
	fmt.Printf("%d topics in forum '%s'\n", store.TopicsCount(), config.ForumUrl)
	forum.Store = store
	logger.Noticef("Created %s forum\n", forum.Title)
	return forum
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

// reads the configuration file from the path specified by
// the config command line flag.
func readConfig(configFile string) error {
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

func main() {
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

	if err := readConfig(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err.Error())
	}

	for _, forumData := range config.Forums {
		f := NewForum(&forumData)
		if err := addForum(f); err != nil {
			log.Fatalf("Failed to add the forum: %s, err: %s\n", f.Title, err.Error())
		}
	}

	if len(appState.Forums) == 0 {
		log.Fatalf("No forums defined in config.json")
	}

	r := mux.NewRouter()
	r.HandleFunc("/", makeTimingHandler(handleMain))
	http.HandleFunc("/s/", makeTimingHandler(handleStatic))
	http.HandleFunc("/img/", makeTimingHandler(handleStaticImg))

	r.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	r.HandleFunc("/login", handleLogin)
	r.HandleFunc("/logout", handleLogout)

	r.HandleFunc("/favicon.ico", serve404)
	r.HandleFunc("/{forum}", makeTimingHandler(handleForum))
	r.HandleFunc("/{forum}/", makeTimingHandler(handleForum))
	r.HandleFunc("/{forum}/rss", makeTimingHandler(handleRss))
	r.HandleFunc("/{forum}/topic", makeTimingHandler(handleTopic))

	//r.HandleFunc("/{forum}/newpost", makeTimingHandler(handleNewPost))

	http.Handle("/", r)
	msg := fmt.Sprintf("Started runing on %s", *httpAddr)
	logger.Noticef(msg)
	println(msg)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")
}
