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

func testStringSliceEq(t *testing.T, s1, s2 []string) {
	if len(s1) != len(s2) {
		t.Fatalf("len(s1) != len(s2) (%d != %d)", len(s1), len(s2))
	}
	for i, _ := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("s1[%d] != s2[%d] (%s != %s)", i, i, s1[i], s2[i])
		}
	}
}

func TestIpConv(t *testing.T) {
	testIpConvOne(t, "127.0.0.1")
	testIpConvOne(t, "27.3.255.238")
	testIpConvOne(t, "hello kitty")

	testMakeInternalUserName(t, "foo", false, "foo")
	testMakeInternalUserName(t, "foo", true, "t:foo")
	testMakeInternalUserName(t, "t:foo", false, "foo")
	testMakeInternalUserName(t, "p:", false, "p")

	testipAddrFromRemoteAddr(t, "foo", "foo")
	testipAddrFromRemoteAddr(t, "[::1]:58292", "[::1]")
	testipAddrFromRemoteAddr(t, "127.0.0.1:856", "127.0.0.1")

	a := []string{"foo", "bar", "go"}
	deleteStringIn(&a, "foo")
	testStringSliceEq(t, a, []string{"go", "bar"})
	deleteStringIn(&a, "go")
	testStringSliceEq(t, a, []string{"bar"})
	deleteStringIn(&a, "baro")
	testStringSliceEq(t, a, []string{"bar"})
	deleteStringIn(&a, "bar")
	testStringSliceEq(t, a, []string{})
}

func testUnCaps(t *testing.T, s, exp string) {
	got := UnCaps(s)
	if got != exp {
		t.Fatalf("\n%#v !=\n%#v (for '%#v')", got, exp, s)
	}
}

func TestUnCaps(t *testing.T) {
	d := []string{
		"FOO", "Foo",
		//"FOO BAR. IS IT ME?\nOR ME", "Foo bar. Is it me?\nOr me",
	}
	for i := 0; i < len(d)/2; i++ {
		testUnCaps(t, d[i*2], d[i*2+1])
	}
}
