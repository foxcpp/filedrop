package filedrop_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/foxcpp/filedrop"
)

var authDB = map[string]bool{
	"foo": true,
	"bar": true,
	"baz": false,
}

func authCallback(r *http.Request) bool {
	return authDB[r.URL.Query().Get("authToken")]
}

func TestAccessDenied(t *testing.T) {
	conf := filedrop.Default
	conf.UploadAuth.Callback = authCallback
	conf.DownloadAuth.Callback = authCallback
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	if !t.Run("upload (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt?authToken=baz","text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	// Access check should be done before existence check to deter scanning.
	if !t.Run("download (fail)", func(t *testing.T) {
		doGETFail(t, c, ts.URL + "/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA/meow.txt?authToken=baz")
	}) {
		t.FailNow()
	}
}

func TestUploadAuth(t *testing.T) {
	conf := filedrop.Default
	conf.UploadAuth.Callback = authCallback
	conf.DownloadAuth.Callback = authCallback
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	if !t.Run("upload", func(t *testing.T) {
		doPOST(t, c, ts.URL + "/filedrop/meow.txt?authToken=foo","text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	if !t.Run("download (fail)", func(t *testing.T) {
		doGETFail(t, c, ts.URL + "/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA?authToken=baz")
	}) {
		t.FailNow()
	}
}