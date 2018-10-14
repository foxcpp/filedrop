package filedrop_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxcpp/filedrop"
	_ "github.com/mattn/go-sqlite3"
)

var TestDBConf = filedrop.DBConfig{
	Driver: "sqlite3",
	DSN:    ":memory:",
}

var file = `Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow 
Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow 
Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow`

func initServ(conf filedrop.Config) *filedrop.Server {
	conf.DB = TestDBConf
	tempDir, err := ioutil.TempDir("", "filedrop-tests-")
	if err != nil {
		panic(err)
	}
	conf.DB.DSN = filepath.Join(tempDir, "index.db")
	conf.StorageDir = tempDir
	serv, err := filedrop.New(conf)
	if err != nil {
		panic(err)
	}
	return serv
}

// Test for correct initialization of server.
func TestNew(t *testing.T) {
	t.Parallel()
	conf := filedrop.Default
	conf.DB = TestDBConf
	tempDir, err := ioutil.TempDir("", "filedrop-tests")
	if err != nil {
		panic(err)
	}
	conf.StorageDir = tempDir

	serv, err := filedrop.New(conf)
	if err != nil {
		t.Error("filedrop.New:", err)
		t.FailNow()
	}
	if err := serv.Close(); err != nil {
		t.Error("s.Close:", err)
		t.FailNow()
	}
}

func doPOST(t *testing.T, c *http.Client, url string, contentType string, reqBody io.Reader) []byte {
	resp, err := c.Post(url, contentType, reqBody)
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
	return body
}

func doPOSTFail(t *testing.T, c *http.Client, url string, contentType string, reqBody io.Reader) int {
	resp, err := c.Post(url, contentType, reqBody)
	if err != nil {
		t.Error("POST:", err)
		t.FailNow()
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		t.Error("POST: HTTP", resp.StatusCode, resp.Status)
		t.FailNow()
	}
	return resp.StatusCode
}

func doGET(t *testing.T, c *http.Client, url string) []byte {
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
	return body
}

func doGETFail(t *testing.T, c *http.Client, url string) int {
	resp, err := c.Get(url)
	if err != nil {
		t.Error("GET:", err)
		t.FailNow()
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		t.Error("GET: HTTP", resp.StatusCode, resp.Status)
		t.FailNow()
	}
	return resp.StatusCode
}

func TestBasicSubmit(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

	t.Log("File URL:", url)
	if !strings.HasSuffix(url, "meow.txt") {
		t.Error("Missing filename suffix on URL")
		t.FailNow()
	}

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

func TestDifferentFilename(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	fileUrl := string(doPOST(t, c, ts.URL+"/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

	t.Log("File URL:", fileUrl)
	if !strings.HasSuffix(fileUrl, "meow.txt") {
		t.Error("Missing filename suffix on URL")
		t.FailNow()
	}
	parsedUrl, err := url.Parse(fileUrl)
	if err != nil {
		t.Error("Url parse failed:", err)
		t.FailNow()
	}
	// Replace last path element (should be filename) with a different one.
	splittenPath := strings.Split(parsedUrl.Path, "/")
	splittenPath = splittenPath[:len(splittenPath)-1]
	splittenPath = append(splittenPath, "meow2.txt")
	parsedUrl.Path = strings.Join(splittenPath, "/")
	fileUrl = parsedUrl.String()

	fileBody := doGET(t, c, fileUrl)
	if string(fileBody) != file {
		t.Log("Got different file!")
		sentHash := sha256.Sum256([]byte(file))
		t.Log("Sent:", hex.EncodeToString(sentHash[:]))
		recvHash := sha256.Sum256(fileBody)
		t.Log("Received:", hex.EncodeToString(recvHash[:]))
		t.FailNow()
	}
}

func TestNonExistent(t *testing.T) {
	// Non-existent file should correctly return 404 code.
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	code := doGETFail(t, c, ts.URL+"/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA/meow2.txt")
	if code != 404 {
		t.Error("GET: HTTP", code)
		t.FailNow()
	}
}

func TestContentTypePreserved(t *testing.T) {
	serv := initServ(filedrop.Default)
	ts := httptest.NewServer(serv)
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop/meow.txt", "text/kitteh", strings.NewReader(file)))

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
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	url := string(doPOST(t, c, ts.URL+"/filedrop/meow.txt", "", strings.NewReader(file)))

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
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
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
	defer os.RemoveAll(serv.Conf.StorageDir)
	defer serv.Close()
	defer ts.Close()
	c := ts.Client()

	testWithPrefix(t, ts, c, "/a/b/c/d/e/f/g")
	testWithPrefix(t, ts, c, "/a/f%20oo/g")
	testWithPrefix(t, ts, c, "")
}
