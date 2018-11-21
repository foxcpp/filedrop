package filedrop_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/filedrop"
)

func TestPerFileMaxUses(t *testing.T) {
	t.Parallel()

	conf := filedrop.Default
	conf.Limits.MaxUses = 2
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	if !t.Run("submit with max-uses=WRONG (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop?max-uses=WRONG", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	if !t.Run("submit with max-uses=3 (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop?max-uses=3", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	var url string
	if !t.Run("submit with max-uses=1", func(t *testing.T) {
		url = string(doPOST(t, c, ts.URL+"/filedrop?max-uses=1", "text/plain", strings.NewReader(file)))
	}) {
		t.FailNow()
	}
	if !t.Run("1 use", func(t *testing.T) {
		doGET(t, c, url)
	}) {
		t.FailNow()
	}
	if !t.Run("2 use", func(t *testing.T) {
		doGET(t, c, url)
	}) {
		t.FailNow()
	}
}

func TestPerFileStoreTime(t *testing.T) {
	t.Parallel()

	conf := filedrop.Default
	conf.Limits.MaxStoreSecs = 5
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	if !t.Run("submit with store-secs=WRONG (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop?store-secs=WRONG", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	if !t.Run("submit with store-secs=15 (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop?store-secs=15", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	var url string
	if !t.Run("submit with store-secs=5", func(t *testing.T) {
		url = string(doPOST(t, c, ts.URL+"/filedrop?store-secs=5", "text/plain", strings.NewReader(file)))
	}) {
		t.FailNow()
	}

	time.Sleep(1 * time.Second)

	if !t.Run("get after 1 second", func(t *testing.T) {
		doGET(t, c, url)
	}) {
		t.FailNow()
	}

	time.Sleep(5 * time.Second)

	if !t.Run("get after 6 seconds (fail)", func(t *testing.T) {
		if code := doGETFail(t, c, url); code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	}) {
		t.FailNow()
	}
}

func TestUnboundedFileLimits(t *testing.T) {
	t.Parallel()

	// Setting no limits in file should allow us to set them to any value in arguments.
	conf := filedrop.Default
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	doPOST(t, c, ts.URL+"/filedrop?store-secs=999999999", "text/plain", strings.NewReader(file))
	doPOST(t, c, ts.URL+"/filedrop?max-uses=999999999", "text/plain", strings.NewReader(file))
}
