// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	"errors"
	_ "fmt"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
	"github.com/kjk/u"
)

// TODO: s3 request have a tendency to block forever so I probably need something
// for that

var backupFreq = 12 * time.Hour
var bucketDelim = "/"

// since we backup twice a day, that should be ~32 days of backups
const MaxBackupsToKeep = 64

type BackupConfig struct {
	AwsAccess string
	AwsSecret string
	Bucket    string
	S3Dir     string
	LocalDir  string
}

// removes "/" if exists and adds delim if missing
func sanitizeDirForList(dir, delim string) string {
	if strings.HasPrefix(dir, "/") {
		dir = dir[1:]
	}
	if !strings.HasSuffix(dir, delim) {
		dir = dir + delim
	}
	return dir
}

func listBackupFiles(config *BackupConfig, max int) (*s3.ListResp, error) {
	auth := aws.Auth{AccessKey: config.AwsAccess, SecretKey: config.AwsSecret}
	b := s3.New(auth, aws.USEast).Bucket(config.Bucket)
	dir := sanitizeDirForList(config.S3Dir, bucketDelim)
	return b.List(dir, bucketDelim, "", max)
}

func listBlobFiles(config *BackupConfig, dir string) ([]s3.Key, error) {
	auth := aws.Auth{AccessKey: config.AwsAccess, SecretKey: config.AwsSecret}
	b := s3.New(auth, aws.USEast).Bucket(config.Bucket)
	ret := make([]s3.Key, 0)
	dir = sanitizeDirForList(dir, bucketDelim)
	marker := ""
	for {
		// note: according to my tests, 1000 is max
		if res, err := b.List(dir, "", marker, 1000); err != nil {
			return nil, err
		} else {
			for _, k := range res.Contents {
				ret = append(ret, k)
			}
			if !res.IsTruncated {
				break
			}
			marker = res.Contents[len(res.Contents)-1].Key
		}
	}
	return ret, nil
}

func s3Del(config *BackupConfig, keyName string) error {
	auth := aws.Auth{AccessKey: config.AwsAccess, SecretKey: config.AwsSecret}
	b := s3.New(auth, aws.USEast).Bucket(config.Bucket)
	return b.Del(keyName)
}

func s3Put(config *BackupConfig, local, remote string, public bool) error {
	localf, err := os.Open(local)
	if err != nil {
		return err
	}
	defer localf.Close()
	localfi, err := localf.Stat()
	if err != nil {
		return err
	}

	auth := aws.Auth{AccessKey: config.AwsAccess, SecretKey: config.AwsSecret}
	b := s3.New(auth, aws.USEast).Bucket(config.Bucket)

	acl := s3.Private
	if public {
		acl = s3.PublicRead
	}

	contType := mime.TypeByExtension(path.Ext(local))
	if contType == "" {
		contType = "binary/octet-stream"
	}

	err = b.PutBucket(acl)
	if err != nil {
		return err
	}
	opts := s3.Options{}
	return b.PutReader(remote, localf, localfi.Size(), contType, acl, opts)
}

// s3Put() likes to fail when putting lots of files in a sequence, so retry once
// for better reliability
func s3PutRetry(config *BackupConfig, local, remote string, public bool) error {
	if err := s3Put(config, local, remote, public); err != nil {
		time.Sleep(100 * time.Millisecond)
		return s3Put(config, local, remote, public)
	}
	return nil
}

// tests if s3 credentials are valid and aborts if aren't
func ensureValidConfig(config *BackupConfig) {
	if !u.PathExists(config.LocalDir) {
		log.Fatalf("Invalid s3 backup: directory to backup %q doesn't exist\n", config.LocalDir)
	}

	if !strings.HasSuffix(config.S3Dir, bucketDelim) {
		config.S3Dir += bucketDelim
	}
	_, err := listBackupFiles(config, 10)
	if err != nil {
		log.Fatalf("Invalid s3 backup: bucket.List failed %s\n", err)
	}
}

// Return true if a backup file with given sha1 content has already been uploaded
// Grabs 10 newest files and checks if sha1 is part of the name, on the theory
// that if the content hasn't changed, the last backup file should have
// the same content, so we don't need to check all files
func alreadyUploaded(config *BackupConfig, sha1 string) bool {
	rsp, err := listBackupFiles(config, 1024)
	if err != nil {
		logger.Errorf("alreadyUploaded(): listBackupFiles() failed with %s", err)
		return false
	}
	for _, key := range rsp.Contents {
		if strings.Contains(key.Key, sha1) {
			return true
		}
	}
	return false
}

// backup file name is in the form:
// apptranslator/121011_1121_c7fedc06cf4b08fef66090eaa0ad7a68dc13a325.zip
// return true if s matches that form
func isBackupFile(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	s = parts[1]
	parts = strings.Split(s, "_")
	if len(parts) != 3 || len(parts[0]) != 6 || len(parts[1]) != 4 {
		return false
	}
	if len(parts[2]) != 40+4 {
		return false
	}
	return strings.HasSuffix(parts[2], ".zip")
}

func deleteOldBackups(config *BackupConfig, maxToKeep int) {
	rsp, err := listBackupFiles(config, 1024)
	if err != nil {
		logger.Errorf("deleteOldBackups(): listBackupFiles() failed with %s", err)
		return
	}
	keys := make([]string, 0)
	for _, key := range rsp.Contents {
		if isBackupFile(key.Key) {
			keys = append(keys, key.Key)
		}
	}
	toDelete := len(keys) - maxToKeep
	if toDelete <= 0 {
		return
	}
	sort.Strings(keys)
	// keys are sorted with the oldest at the beginning of keys array, so we
	// delete those first
	for i := 0; i < toDelete; i++ {
		key := keys[i]
		if err = s3Del(config, key); err != nil {
			logger.Noticef("deleteOldBackups(): failed to delete %s, error: %s", key, err)
		} else {
			logger.Noticef("deleteOldBackups(): deleted %s", key)
		}
	}
}

func copyBlobs(config *BackupConfig) error {
	blobsDir := filepath.Join(config.LocalDir, "blobs")
	blobsS3Dir := filepath.Join(config.S3Dir, "blobs")

	existing := 0
	copied := 0
	blobFilesInS3 := make(map[string]bool)

	if keys, err := listBlobFiles(config, blobsS3Dir); err != nil {
		logger.Errorf("listBlobFiles() failed with %s\n", err)
		return err
	} else {
		for _, key := range keys {
			// the key values do not include '/' at the beginning, add it for
			// the benefit of the later check
			s := "/" + key.Key
			blobFilesInS3[s] = true
		}
	}

	err := filepath.Walk(blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Errorf("WalkFunc() received err %s from filepath.Wath()", err)
			return err
		}
		isDir, err := u.PathIsDir(path)
		if err != nil {
			logger.Errorf("PathIsDir() for %s failed with %s\n", path, err)
			return err
		}
		if isDir {
			return nil
		}

		idx := strings.Index(path, "/blobs/")
		if idx == -1 {
			logger.Errorf("copyBlobs(): unknown file %q", path)
			return errors.New("unknown file")
		}
		file := path[idx+len("/blobs/"):]
		s3Path := filepath.Join(blobsS3Dir, file)
		if _, ok := blobFilesInS3[s3Path]; ok {
			existing++
			return nil
		}

		if err = s3PutRetry(config, path, s3Path, true); err != nil {
			logger.Errorf("s3Put of %q to %q failed with %s", path, s3Path, err)
			return err
		} else {
			logger.Noticef("copyBlobs(): s3Put %q as %q\n", path, s3Path)
		}
		copied++
		return nil
	})
	logger.Noticef("copyBlobs(): skipped %d existing files, copied %d files\n", existing, copied)
	return err
}

func doBackup(config *BackupConfig) {
	startTime := time.Now()
	forumDir := filepath.Join(config.LocalDir, "forum")
	if err := copyBlobs(config); err != nil {
		logger.Errorf("doBackup(): copyBlobs() failed with %s", err)
		return
	}

	zipLocalPath := filepath.Join(os.TempDir(), "fofou-tmp-backup.zip")
	// TODO: do I need os.Remove() won't os.Create() over-write the file anyway?
	os.Remove(zipLocalPath) // remove before trying to create a new one, just in cased
	err := u.CreateZipWithDirContent(zipLocalPath, forumDir)
	defer os.Remove(zipLocalPath)
	if err != nil {
		return
	}
	sha1, err := u.Sha1HexOfFile(zipLocalPath)
	if err != nil {
		return
	}
	if alreadyUploaded(config, sha1) {
		dur := time.Now().Sub(startTime)
		logger.Noticef("s3 backup not done because data (%s) didn't changed, took %.2f secs", sha1, dur.Seconds())
		return
	}
	timeStr := time.Now().Format("060102_1504_")
	zipS3Path := path.Join(config.S3Dir, timeStr+sha1+".zip")

	if err = s3Put(config, zipLocalPath, zipS3Path, true); err != nil {
		logger.Errorf("s3Put of %q to %q failed with %s", zipLocalPath, zipS3Path, err)
		return
	}

	deleteOldBackups(config, MaxBackupsToKeep)

	dur := time.Now().Sub(startTime)
	logger.Noticef("s3 backup of %q to %q took %.2f secs", zipLocalPath, zipS3Path, dur.Seconds())
}

func BackupLoop(config *BackupConfig) {
	ensureValidConfig(config)
	for {
		doBackup(config)
		time.Sleep(backupFreq)
	}
}
