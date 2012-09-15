package main

import (
	"time"
)

// TODO: this will be stored in a cookie
/*type FofouUser struct {
  User string
  Cookie string
  Email string
  Name string
  Homepage string
  RemeberMe bool
}
*/

type StorageItem struct {
	Id int
}

type Topic struct {
	StorageItem
	Subject   string
	CreatedOn time.Time
	CreatedBy string
	IsDeleted bool
}

type Post struct {
	StorageItem
	Topic        int // refers to Topic.Id
	CreatedOn    time.Time
	MessageRef   string
	UserIpAddr   string
	UserName     string
	UserEmail    string
	UserHomepage string
}

type DeleteUndelete struct {
	StorageItem
	ItemDeletedUndeleted int  // refers to Topic.Id or Post.Id
	IsDeleted            bool // true means deleted, false means undeleted
}
