package filedrop_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/filedrop"
)

func TestGlobalMaxUses(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxUses = 2
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL + "/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

	// First should succeed (1 use).
	doGET(t, c, url)
	// Should succeed too (2 use).
	doGET(t, c, url)

	// Third use should fail.
	if code := doGETFail(t, c, url); code != 404 {
		t.Error("GET: HTTP", code)
		t.FailNow()
	}
}

func TestGlobalMaxFileSize(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxFileSize = uint(len(file) - 20)
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	// Submit should fail.
	doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt", "text/plain", strings.NewReader(file))
}

func TestGlobalMaxStoreTime(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxStoreSecs = 10
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL + "/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

	time.Sleep(5 * time.Second)

	// Should be still available.
	doGET(t, c, url)

	time.Sleep(6 * time.Second)

	// Should not longer be available.
	if code := doGETFail(t, c, url); code != 404 {
		t.Error("GET: HTTP", code)
		t.FailNow()
	}
}