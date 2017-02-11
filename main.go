// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/gorilla/securecookie"
	"github.com/kjk/u"
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
		CookieAuthKeyHexStr     *string
		CookieEncrKeyHexStr     *string
		AnalyticsCode           *string
		AwsAccess               *string
		AwsSecret               *string
		S3BackupBucket          *string
		S3BackupDir             *string
	}{
		&oauthClient.Credentials,
		nil, nil,
		nil,
		nil, nil,
		nil, nil,
	}

	forums = make([]*ForumConfig, 0)

	logger        *ServerLogger
	cookieAuthKey []byte
	cookieEncrKey []byte
	secureCookie  *securecookie.SecureCookie

	dataDir string

	appState = AppState{
		Users:  make([]*User, 0),
		Forums: make([]*Forum, 0),
	}

	alwaysLogTime = true
)

// ForumConfig is a static configuration of a single forum
type ForumConfig struct {
	Title      string
	ForumUrl   string
	WebsiteUrl string
	Tagline    string
	DataDir    string
	// we authenticate only with Twitter, this is the twitter user name
	// of the admin user
	AdminTwitterUser string
	Disabled         bool
	BannedIps        *[]string
	BannedWords      *[]string

	SidebarTmpl *template.Template
}

// User describes a user
type User struct {
	Login string
}

// Forum describes forum
type Forum struct {
	ForumConfig
	Store *Store
}

// AppState describes state of the app
type AppState struct {
	Users  []*User
	Forums []*Forum
}

// StringEmpty returns true if string is empty
func StringEmpty(s *string) bool {
	return s == nil || 0 == len(*s)
}

// S3BackupEnabled returns true if backup to s3 is enabled
func S3BackupEnabled() bool {
	if !*inProduction {
		logger.Notice("s3 backups disabled because not in production")
		return false
	}
	if StringEmpty(config.AwsAccess) {
		logger.Notice("s3 backups disabled because AwsAccess not defined in config.json\n")
		return false
	}
	if StringEmpty(config.AwsSecret) {
		logger.Notice("s3 backups disabled because AwsSecret not defined in config.json\n")
		return false
	}
	if StringEmpty(config.S3BackupBucket) {
		logger.Notice("s3 backups disabled because S3BackupBucket not defined in config.json\n")
		return false
	}
	if StringEmpty(config.S3BackupDir) {
		logger.Notice("s3 backups disabled because S3BackupDir not defined in config.json\n")
		return false
	}
	return true
}

func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}

	dirsToCheck := []string{
		"/data",
		filepath.Join("..", "..", "data"),   // old on the server
		u.ExpandTildeInPath("~/data/fofou"), // locally
	}

	for _, dir := range dirsToCheck {
		if u.PathExists(dir) {
			dataDir = dir
			return dataDir
		}
	}
	log.Fatalf("data directory (%q) doesn't exist", dirsToCheck)
	return ""
}

// NewForum creates new forum
func NewForum(config *ForumConfig) *Forum {
	forum := &Forum{ForumConfig: *config}
	sidebarTmplPath := filepath.Join("forums", fmt.Sprintf("%s_sidebar.html", forum.ForumUrl))
	if !u.PathExists(sidebarTmplPath) {
		panic(fmt.Sprintf("sidebar template %s for forum %s doesn't exist", sidebarTmplPath, forum.ForumUrl))
	}

	forum.SidebarTmpl = template.Must(template.ParseFiles(sidebarTmplPath))

	store, err := NewStore(getDataDir(), config.DataDir)
	if err != nil {
		logger.Errorf("NewStore('%s', '%s') failed with '%s'\n", getDataDir(), config.DataDir)
		panic("failed to create store for a forum")
	}
	logger.Noticef("%d topics, %d posts in forum %q", store.TopicsCount(), store.PostsCount(), config.ForumUrl)
	forum.Store = store
	return forum
}

func findForum(forumURL string) *Forum {
	for _, f := range appState.Forums {
		if f.ForumUrl == forumURL {
			return f
		}
	}
	return nil
}

func forumAlreadyExists(siteURL string) bool {
	return nil != findForum(siteURL)
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
		return fmt.Errorf("Forum has invalid field %q", invalidField)
	}
	if forumAlreadyExists(forum.ForumUrl) {
		return errors.New("Forum already exists")
	}
	// verify BannedIps are valid regexpes
	banned := forum.BannedIps
	if banned != nil {
		for _, s := range *banned {
			_, err := regexp.Compile(s)
			if err != nil {
				log.Fatalf("%q is not a valid regexp, err: %s", s, err)
			}
		}
	}
	appState.Forums = append(appState.Forums, forum)
	return nil
}

// DoSidebarTemplate renders sidebar template
func DoSidebarTemplate(forum *Forum, isAdmin bool) string {
	n := forum.Store.GetBlockedIpsCount()
	model := struct {
		IsAdmin         bool
		BlockedIpsCount int
	}{
		IsAdmin:         isAdmin,
		BlockedIpsCount: n,
	}

	var buf bytes.Buffer
	tmpl := forum.SidebarTmpl

	s := ""
	if err := tmpl.Execute(&buf, model); err != nil {
		logger.Errorf("Failed to execute sidebar template for forum %q error: %s", forum.ForumUrl, err)
	} else {
		s = string(buf.Bytes())
	}
	return s
}

func isTopLevelURL(url string) bool {
	return 0 == len(url) || "/" == url
}

func userIsAdmin(f *Forum, cookie *SecureCookieValue) bool {
	return cookie.TwitterUser == f.AdminTwitterUser
}

// reads forums/*_config.json files
func readForumConfigs(configDir string) error {
	pat := filepath.Join(configDir, "*_config.json")
	files, err := filepath.Glob(pat)
	if err != nil {
		return err
	}
	if files == nil {
		return errors.New("No forums configured!")
	}
	for _, configFile := range files {
		var forum ForumConfig
		b, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(b, &forum)
		if err != nil {
			return err
		}
		if !forum.Disabled {
			forums = append(forums, &forum)
		}
	}
	if len(forums) == 0 {
		return errors.New("All forums are disabled!")
	}
	return nil
}

// reads the configuration file from the path specified by
// the config command line flag.
func readConfig(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("%s config file doesn't exist. Read readme.md for config instructions", configFile)
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
		fmt.Printf("CookieAuthKeyHexStr and CookieEncrKeyHexStr are invalid or missing in %q\nYou can use the following random values:\n", configFile)
		auth := securecookie.GenerateRandomKey(32)
		encr := securecookie.GenerateRandomKey(32)
		fmt.Printf("CookieAuthKeyHexStr: %s\nCookieEncrKeyHexStr: %s\n", hex.EncodeToString(auth), hex.EncodeToString(encr))
	}
	// TODO: somehow verify twitter creds
	return err
}

func getReferer(r *http.Request) string {
	return r.Header.Get("Referer")
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
			logger.Noticef("%q took %f seconds to serve", url, duration.Seconds())
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

	useStdout := !*inProduction
	logger = NewServerLogger(256, 256, useStdout)

	rand.Seed(time.Now().UnixNano())

	if err := readConfig(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err)
	}

	if err := readForumConfigs("forums"); err != nil {
		log.Fatalf("Failed to read forum configs, err: %s", err)
	}

	for _, forumData := range forums {
		f := NewForum(forumData)
		if err := addForum(f); err != nil {
			log.Fatalf("Failed to add the forum: %s, err: %s\n", f.Title, err)
		} else {
			fmt.Printf("added forum %s\n", f.ForumUrl)
		}
	}

	if len(appState.Forums) == 0 {
		log.Fatalf("No forums defined in config.json")
	}

	backupConfig := &BackupConfig{
		AwsAccess: *config.AwsAccess,
		AwsSecret: *config.AwsSecret,
		Bucket:    *config.S3BackupBucket,
		S3Dir:     *config.S3BackupDir,
		LocalDir:  getDataDir(),
	}

	if S3BackupEnabled() {
		go BackupLoop(backupConfig)
	}

	InitHttpHandlers()
	logger.Noticef(fmt.Sprintf("Started runing on %s", *httpAddr))
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
