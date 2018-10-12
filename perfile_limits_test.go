package filedrop_test

import (
	"net/http/httptest"
	"os"
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
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	if !t.Run("submit with max-uses=WRONG (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop/meow.txt?max-uses=WRONG", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	if !t.Run("submit with max-uses=3 (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL + "/filedrop/meow.txt?max-uses=3", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	var url string
	if !t.Run("submit with max-uses=1", func(t *testing.T) {
		url = string(doPOST(t, c, ts.URL+"/filedrop/meow.txt?max-uses=1", "text/plain", strings.NewReader(file)))
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
	conf := filedrop.Default
	conf.Limits.MaxStoreSecs = 10
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	if !t.Run("submit with store-secs=WRONG (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop/meow.txt?store-secs=WRONG", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	if !t.Run("submit with store-secs=15 (fail)", func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop/meow.txt?store-secs=15", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	var url string
	if !t.Run("submit with store-secs=10", func(t *testing.T) {
		url = string(doPOST(t, c, ts.URL+"/filedrop/meow.txt?store-secs=10", "text/plain", strings.NewReader(file)))
	}) {
		t.FailNow()
	}

	time.Sleep(5 * time.Second)

	if !t.Run("get after 5 seconds", func(t *testing.T) {
		doGET(t, c, url)
	}) {
		t.FailNow()
	}

	time.Sleep(6 * time.Second)

	if !t.Run("get after 11 seconds (fail)", func(t *testing.T) {
		if code := doGETFail(t, c, url); code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	}) {
		t.FailNow()
	}
}
