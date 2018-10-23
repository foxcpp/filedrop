package filedrop_test

import (
	"net/http/httptest"
	"os"
	"strconv"
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
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(file)))

	t.Run("1 use", func(t *testing.T) {
		doGET(t, c, url)
	})
	t.Run("2 use", func(t *testing.T) {
		doGET(t, c, url)
	})
	t.Run("3 use (fail)", func(t *testing.T) {
		if code := doGETFail(t, c, url); code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	})
}

func TestGlobalMaxFileSize(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxFileSize = uint(len(file) - 20)
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	t.Log("Max size:", conf.Limits.MaxFileSize, "bytes")
	if !t.Run("submit with size "+strconv.Itoa(len(file)), func(t *testing.T) {
		doPOSTFail(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(file))
	}) {
		t.FailNow()
	}

	strippedFile := file[:25]
	if !t.Run("submit with size "+strconv.Itoa(len(strippedFile)), func(t *testing.T) {
		doPOST(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(strippedFile))
	}) {
		t.FailNow()
	}
}

func TestGlobalMaxStoreTime(t *testing.T) {
	conf := filedrop.Default
	conf.Limits.MaxStoreSecs = 3
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	var url string
	if !t.Run("submit", func(t *testing.T) {
		url = string(doPOST(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(file)))
	}) {
		t.FailNow()
	}

	time.Sleep(1 * time.Second)

	if !t.Run("get after 1 second", func(t *testing.T) {
		doGET(t, c, url)
	}) {
		t.FailNow()
	}

	time.Sleep(3 * time.Second)

	if !t.Run("get after 3 seconds (fail)", func(t *testing.T) {
		if code := doGETFail(t, c, url); code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	}) {
		t.FailNow()
	}
}
