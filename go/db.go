package main

import (
	"time"
	"store"
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

type Topic struct {
	store.StoreItem
	Subject   string
	CreatedOn time.Time
	CreatedBy string
	IsDeleted bool
}

type Post struct {
	store.StoreItem
	Topic        int // refers to Topic.Id
	CreatedOn    time.Time
	MessageRef   string
	UserIpAddr   string
	UserName     string
	UserEmail    string
	UserHomepage string
}

type DeleteUndelete struct {
	store.StoreItem
	ItemDeletedUndeleted int  // refers to Topic.Id or Post.Id
	IsDeleted            bool // true means deleted, false means undeleted
}
