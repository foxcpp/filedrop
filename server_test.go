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
	DSN: ":memory:",
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
	if resp.StatusCode / 100 != 2 {
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
	if resp.StatusCode / 100 == 2 {
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
	if resp.StatusCode / 100 != 2 {
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
	if resp.StatusCode / 100 == 2 {
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

	url := string(doPOST(t, c, ts.URL + "/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

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

	fileUrl := string(doPOST(t, c, ts.URL + "/filedrop/meow.txt", "text/plain", strings.NewReader(file)))

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

	code := doGETFail(t, c, ts.URL + "/filedrop/AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA/meow2.txt")
	if code != 404 {
		t.Error("GET: HTTP", code)
		t.FailNow()
	}
}
