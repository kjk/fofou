package store

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type Store struct {
}

type StoreItem struct {
	Id int
}

var (
	nilTime       time.Time
	timeType      reflect.Type = reflect.TypeOf(nilTime)
	nilStoreItem  StoreItem
	storeItemType reflect.Type = reflect.TypeOf(nilStoreItem)
)

// An encodeState encodes JSON into a bytes.Buffer.
type encodeState struct {
	bytes.Buffer // accumulated output
	scratch      [64]byte
}

func (s *Store) LoadStringBySha1(sha1 string) (string, error) {
	return "", nil
}

// write str to a file named ${dir}/xx/yy/${sha1}.txt
// where xx is sha1[0:2] and yy is sha1[2:4]
func saveStringUnderSha1(str, dir string) (string, error) {
	h := sha1.New()
	io.WriteString(h, str)
	sha1 := hex.EncodeToString(h.Sum(nil))
	dir1 := sha1[0:2]
	dir2 := sha1[2:4]
	path := filepath.Join(dir, dir1, dir2)
	err := os.MkdirAll(path, 0666)
	if err != nil {
		return sha1, err
	}
	fileName := sha1 + ".txt"
	path = filepath.Join(path, fileName)
	err = ioutil.WriteFile(path, []byte(str), 0666)
	return sha1, err
}

func serializeString(name, v string) error {
	// since we separate records by \n and I don't want to de escaping, we
	// go for simplicity and remove potential \n in the value
	v = strings.Replace(v, "\n", "", -1)
	fmt.Printf("%s: %s\n", name, v)
	return nil
}

func serializeRefString(name, v string) error {
	// TODO: save v under ${sha1}.txt name and store sha1 as value
	v = strings.Replace(v, "\n", "", -1)
	fmt.Printf("%s: %s\n", name, v)
	return nil
}

func serializeInt(name string, v int64) error {
	fmt.Printf("%s: %d\n", name, v)
	return nil
}

func serializeBool(name string, v bool) error {
	if v {
		fmt.Printf("%s: true\n", name)
	} else {
		fmt.Printf("%s: false\n", name)
	}
	return nil
}

func serializeTime(name string, v time.Time) error {
	fmt.Printf("%s: %s\n", name, v.Format(time.RFC3339))
	return nil
}

func serializeValue(name string, v reflect.Value) error {
	switch v.Kind() {

	case reflect.Bool:
		return serializeBool(name, v.Bool())

	case reflect.Int:
		return serializeInt(name, v.Int())

	case reflect.String:
		if strings.HasSuffix(name, "Ref") {
			return serializeRefString(name, v.String())
		} else {
			return serializeString(name, v.String())
		}
		return nil
	}
	if v.Type() == timeType {
		return serializeTime(name, v.Interface().(time.Time))
	}
	fmt.Printf("field name: %s type: %v\n", name, v.Type())
	panic("Unknown type")
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
		if i == 0 {
			if elem.Type() != storeItemType {
				fmt.Printf("enumStructFields(): first field is %s and should be StoreItem\n", sf.Name)
			}
			si, ok := elem.Interface().(StoreItem)
			if !ok {
				panic("First field should always be StoreItem")
			}
			// Using lower-case name for the id so that it won't conflict
			// with a struct field Id
			serializeInt("id", int64(si.Id))
			continue
		}
		serializeValue(sf.Name, elem)
	}
	fmt.Printf("\n")
}

/*
func testEnumFields() {
	v := &Topic{
		Subject:   "This is subject",
		CreatedOn: time.Now(),
		CreatedBy: "Created by me",
		IsDeleted: false}
	v.Id = 1
	serializeStruct(reflect.ValueOf(*v))
}*/

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
