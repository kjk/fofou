package main

import (
	"time"
	"reflect"
	"bytes"
	_ "runtime"
	_ "sync"
	"fmt"
	"strings"
	"crypto/sha1"
	"io"
	"encoding/hex"
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

type Store struct {

}

// An encodeState encodes JSON into a bytes.Buffer.
type encodeState struct {
	bytes.Buffer // accumulated output
	scratch      [64]byte
}

func (s *Store) LoadStringBySha1(sha1 string) (string, error) {
	return "", nil
}

func (s *Store) saveStringUnderSha1(str, dir string) (string, error) {
	h := sha1.New()
	io.WriteString(h, str)
	sha1 := hex.EncodeToString(h.Sum(nil))
	// TODO: write to a file named sha1 in dir xx/yy/sha1
	return sha1, nil
}

func serializeString(name, v string) {
	// since we separate records by \n and I don't want to de escaping, we
	// go for simplicity and remove potential \n in the value
	v = strings.Replace(v, "\n", "", -1)
	fmt.Printf("%s: %s\n", name, v)
}

func serializeRefString(name, v string) {
	// TODO: save v under ${sha1}.txt name and store sha1 as value
	v = strings.Replace(v, "\n", "", -1)
	fmt.Printf("%s: %s\n", name, v)
}

func serializeInt(name string, v int64) {
	fmt.Printf("%s: %d\n", name, v)
}

func serializeBool(name string, v bool) {
	if (v) {
		fmt.Printf("%s: true\n", name)
	} else {
		fmt.Printf("%s: false\n", name)
	}
}

func serializeTime(name string, v time.Time) {
	fmt.Printf("%s: %s\n", name, v.Format(time.RFC3339))
}

func serializeStruct(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		fmt.Printf("enumStructFields(): v is not a struct but %v\n", v.Kind())
		return
	}
	t := v.Type()
	n := v.NumField()
	for i := 0; i < n; i++ {
		sf := t.Field(i) // StructField
		elem := v.Field(i)
		st := sf.Type
		n := sf.Name
		stn := st.Name()
		if i == 0 {
			if sf.Name != "StorageItem" {
				fmt.Printf("enumStructFields(): first field is %s and should be StorageItem\n", sf.Name)
			}
			i := elem.Interface()
			si := i.(StorageItem)
			serializeInt("Id", int64(si.Id))
			continue
		}
		if "Id" == sf.Name {
			panic("Field name shouldn't be 'Id'")
		}
		// TODO: use kind
		if "string" == stn {
			if strings.HasSuffix(sf.Name, "Ref") {
				serializeRefString(sf.Name, elem.String())
			} else {
				serializeString(sf.Name, elem.String())
			}
		} else if "int" == stn {
			serializeInt(sf.Name, elem.Int())
		} else if "bool" == stn {
			serializeBool(sf.Name, elem.Bool())
		} else if "Time" == stn {
			i := elem.Interface()
			time := i.(time.Time)
			serializeTime(sf.Name, time)
		} else {
			fmt.Printf("field %d, name: %s type: %s\n", i, n, stn)
		}
	}
	fmt.Printf("\n")
}

func testEnumFields() {
	v := &Topic{
		Subject: "This is subject",
		CreatedOn: time.Now(),
		CreatedBy: "Created by me",
		IsDeleted: false}
	v.Id = 1
	serializeStruct(reflect.ValueOf(*v))
}

/*
func Marshall(v interface{}, id int) ([]byte, error) {
	e := &encodeState{}
	err := e.marshal(v, id)
	if err != nil {
		return nil, err
	}
	return e.Bytes(), nil
}

func (e *encodeState) marshal(v interface{}, id int) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	e.reflectValue(reflect.ValueOf(v))
	return nil
}

type encodeField struct {
	i int // /ield index in struct
	name string
}

var (
	typeCacheLock sync.RWMutex
	encodeFieldsCache = make(map[reflect.Type][]encodeField)
)

func encodeFields(t reflect.Type) []encodeField {
	typeCacheLock.RLock()
	fs, ok := encodeFieldsCache[t]
	typeCacheLock.RUnlock()
	if ok {
		return fs
	}
}

func (e *encodeState) reflectValue(v reflect.Value) {

}

*/
