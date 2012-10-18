// This code is in Public Domain. Take all the code you want, I'll just write more.
package main

import (
	_ "fmt"
	"testing"
)

func testIpConvOne(t *testing.T, s string) {
	internal := ipAddrToInternal(s)
	orig := ipAddrInternalToOriginal(internal)
	if s != orig {
		t.Fatalf("'%s' != '%s'", s, orig)
	}
}

func testMakeInternalUserName(t *testing.T, given string, twitter bool, expected string) {
	res := MakeInternalUserName(given, twitter)
	if res != expected {
		t.Fatalf("'%s' != '%s'", res, expected)
	}
}

func testipAddrFromRemoteAddr(t *testing.T, s, expected string) {
	res := ipAddrFromRemoteAddr(s)
	if res != expected {
		t.Fatalf("'%s' != '%s'", res, expected)
	}
}

func TestIpConv(t *testing.T) {
	testIpConvOne(t, "127.0.0.1")
	testIpConvOne(t, "hello kitty")

	testMakeInternalUserName(t, "foo", false, "foo")
	testMakeInternalUserName(t, "foo", true, "t:foo")
	testMakeInternalUserName(t, "t:foo", false, "foo")
	testMakeInternalUserName(t, "p:", false, "p")

	testipAddrFromRemoteAddr(t, "foo", "foo")
	testipAddrFromRemoteAddr(t, "[::1]:58292", "[::1]")
	testipAddrFromRemoteAddr(t, "127.0.0.1:856", "127.0.0.1")
}
