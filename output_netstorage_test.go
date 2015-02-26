package main

import (
	"net/http"
	"testing"
)

func TestNetstorageOutput_Auth(t *testing.T) {
	r, _ := http.NewRequest("PUT", "http://thenetstorages.com", nil)
	tests := []struct {
		keyName  string
		secret   string
		id       string
		filename string
		unixTime int64
		header   map[string]string
	}{
		{"key1", "abcdefghij", "382644692", "dir1/dir2/file.html", 1280000000, map[string]string{
			"X-Akamai-ACS-Auth-Sign": "47+XeqwsRefh88EjBITmviegbJQ1DczLKT2inWZQx9s=",
			"X-Akamai-ACS-Auth-Data": "5, 0.0.0.0, 0.0.0.0, 1280000000, 382644692, key1",
		}},
	}
	for _, test := range tests {
		o := NetstorageOutput{NetstorageKeyName: test.keyName, NetstorageSecret: test.secret}
		o.auth(r, test.id, test.filename, test.unixTime)
		for name, expected := range test.header {
			if actual := r.Header.Get(name); actual != expected {
				t.Errorf("%s=%q, want %q", name, actual, expected)
			}
		}
	}
}
