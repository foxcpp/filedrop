package filedrop_test

import (
	"net/http"
	"net/http/httptest"
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
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	// Upload should fail.
	doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt?authToken=baz","text/plain", strings.NewReader(file))

	// Download too. Access check should be done before existence to deter scanning.
	doGET(t, c, ts.URL + "/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA/meow.txt?authToken=baz")
}

func TestUploadAuth(t *testing.T) {
	conf := filedrop.Default
	conf.UploadAuth.Callback = authCallback
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	// Upload should succeed.
	doPOST(t, c, ts.URL + "/filedrop/meow.txt?authToken=baz","text/plain", strings.NewReader(file))

	// But download no.
	doGETFail(t, c, ts.URL + "/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA?authToken=foo")
}