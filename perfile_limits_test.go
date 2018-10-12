package filedrop_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/filedrop"
)

func TestPerFileMaxUses(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxUses = 2
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	// Submit should with value higher than 2 should fail.
	doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt?max-uses=3", "text/plain", strings.NewReader(file))

	// Submit should with value lower or equal to 2 should succeed.
	url := string(doPOST(t, c, ts.URL + "/filedrop/meow.txt?max-uses=1", "text/plain", strings.NewReader(file)))

	// 1 use.
	doGET(t, c, url)

	// 2 use. Should fail.
	doGETFail(t, c, url)
}

func TestPerFileStoreTime(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxStoreSecs = 10
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	// Submit with value bigger than 10 should fail.
	doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt?store-time=15", "text/plain", strings.NewReader(file))

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
