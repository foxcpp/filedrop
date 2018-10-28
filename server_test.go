package filedrop_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/filedrop"
	_ "github.com/mattn/go-sqlite3"
)

func TestBasicSubmit(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(file)))

	t.Log("File URL:", url)

	fileBody := doGET(t, c, url)
	if string(fileBody) != file {
		t.Log("Got different file!")
		sentHash := sha256.Sum256([]byte(file))
		t.Log("Sent:", hex.EncodeToString(sentHash[:]))
		recvHash := sha256.Sum256(fileBody)
		t.Log("Received:", hex.EncodeToString(recvHash[:]))
		t.FailNow()
	}
}

func TestFakeFilename(t *testing.T) {
	conf := filedrop.Default
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	fileUrl := string(doPOST(t, c, ts.URL+"/filedrop", "text/plain", strings.NewReader(file)))
	t.Log("File URL:", fileUrl)

	t.Run("without fake filename", func(t *testing.T) {
		doGET(t, c, fileUrl)
	})
	t.Run("with fake filename (meow.txt)", func(t *testing.T) {
		doGET(t, c, fileUrl+"/meow.txt")
	})
}

func TestNonExistent(t *testing.T) {
	// Non-existent file should correctly return 404 code.
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	t.Run("non-existent UUID in path", func(t *testing.T) {
		code := doGETFail(t, c, ts.URL+"/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA")
		if code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	})

	t.Run("no UUID in path", func(t *testing.T) {
		code := doGETFail(t, c, ts.URL+"/filedrop")
		if code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	})

	t.Run("invalid UUID in path", func(t *testing.T) {
		code := doGETFail(t, c, ts.URL+"/filedrop/IAMINVALIDUUID")
		if code != 404 {
			t.Error("GET: HTTP", code)
			t.FailNow()
		}
	})
}

func TestContentTypePreserved(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop", "text/kitteh", strings.NewReader(file)))

	t.Log("File URL:", url)

	resp, err := c.Get(url)
	if err != nil {
		t.Error("GET:", err)
		t.FailNow()
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("ioutil.ReadAll:", err)
		t.FailNow()
	}
	if resp.StatusCode/100 != 2 {
		t.Error("GET: HTTP", resp.Status)
		t.Error("Body:", string(body))
		t.FailNow()
	}
	if resp.Header.Get("Content-Type") != "text/kitteh" {
		t.Log("Mismatched content type:")
		t.Log("\tWanted: 'text/kitteh'")
		t.Log("\tGot:", "'"+resp.Header.Get("Content-Type")+"'")
		t.Fail()
	}
}

func TestNoContentType(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop", "", strings.NewReader(file)))

	t.Log("File URL:", url)

	resp, err := c.Get(url)
	if err != nil {
		t.Error("GET:", err)
		t.FailNow()
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("ioutil.ReadAll:", err)
		t.FailNow()
	}
	if resp.StatusCode/100 != 2 {
		t.Error("GET: HTTP", resp.Status)
		t.Error("Body:", string(body))
		t.FailNow()
	}
	t.Log("Got:", "'"+resp.Header.Get("Content-Type")+"'")
}

func TestHTTPSDownstream(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	t.Run("X-HTTPS-Downstream=1", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL, strings.NewReader(file))
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		req.Header.Set("X-HTTPS-Downstream", "1")
		resp, err := c.Do(req)
		if err != nil {
			t.Error("POST:", err)
			t.FailNow()
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error("ioutil.ReadAll:", err)
			t.FailNow()
		}
		if resp.StatusCode/100 != 2 {
			t.Error("POST: HTTP", resp.StatusCode, resp.Status)
			t.Error("Body:", string(body))
			t.FailNow()
		}
		if !strings.HasPrefix(string(body), "https") {
			t.Error("Got non-HTTPS URl with X-HTTPS-Downstream=1")
			t.FailNow()
		}
	})
	t.Run("X-HTTPS-Downstream=0", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL, strings.NewReader(file))
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		req.Header.Set("X-HTTPS-Downstream", "0")
		resp, err := c.Do(req)
		if err != nil {
			t.Error("POST:", err)
			t.FailNow()
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error("ioutil.ReadAll:", err)
			t.FailNow()
		}
		if resp.StatusCode/100 != 2 {
			t.Error("POST: HTTP", resp.StatusCode, resp.Status)
			t.Error("Body:", string(body))
			t.FailNow()
		}
		if !strings.HasPrefix(string(body), "http") {
			t.Error("Got non-HTTP URL with X-HTTPS-Downstream=0")
			t.FailNow()
		}
	})
}

func testWithPrefix(t *testing.T, ts *httptest.Server, c *http.Client, prefix string) {
	var URL string
	t.Run("submit with prefix "+prefix, func(t *testing.T) {
		URL = string(doPOST(t, c, ts.URL+prefix+"/meow.txt", "text/plain", strings.NewReader(file)))
	})

	if !strings.Contains(URL, prefix) {
		t.Errorf("Result URL doesn't contain prefix %v: %v", prefix, URL)
		t.FailNow()
	}

	if URL != "" {
		t.Run("get with "+prefix, func(t *testing.T) {
			body := doGET(t, c, URL)
			if string(body) != file {
				t.Error("Got different file!")
				t.FailNow()
			}
		})
	}
}

func TestPrefixAgnostic(t *testing.T) {
	// Server should be able to handle requests independently
	// from full URL.
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	testWithPrefix(t, ts, c, "/a/b/c/d/e/f/g")
	testWithPrefix(t, ts, c, "/a/f%20oo/g")
	testWithPrefix(t, ts, c, "")
}

func TestCleanup(t *testing.T) {
	conf := filedrop.Default
	conf.CleanupIntervalSecs = 1
	serv := initServ(conf)
	ts := httptest.NewServer(serv)
	defer cleanServ(serv)
	defer ts.Close()
	c := ts.Client()

	URL := string(doPOST(t, c, ts.URL+"/filedrop?store-secs=1", "text/plain", strings.NewReader(file)))
	splittenURL := strings.Split(URL, "/")
	UUID := splittenURL[len(splittenURL)-1]
	time.Sleep(2 * time.Second)

	_, err := os.Stat(filepath.Join(serv.Conf.StorageDir, UUID))
	if err == nil || !os.IsNotExist(err) {
		t.Error("Wanted 'no such file or directory', got:", err)
		t.FailNow()
	}
}
