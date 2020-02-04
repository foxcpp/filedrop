package filedrop_test

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/foxcpp/filedrop"
	_ "github.com/mattn/go-sqlite3"
)

var TestDB = os.Getenv("TEST_DB")
var TestDSN = os.Getenv("TEST_DSN")

var file = `Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow 
Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow 
Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow Meow`

func initServ(conf filedrop.Config) *filedrop.Server {
	tempDir, err := ioutil.TempDir("", "filedrop-tests-")
	if err != nil {
		panic(err)
	}
	conf.StorageDir = tempDir

	conf.DB.Driver = TestDB
	conf.DB.DSN = TestDSN

	// This is meant for DB debugging.
	if TestDB == "" || TestDSN == "" {
		log.Println("Using sqlite3 DB in temporary directory.")
		conf.DB.Driver = "sqlite3"
		conf.DB.DSN = filepath.Join(tempDir, "index.db")
	}

	serv, err := filedrop.New(conf)
	if err != nil {
		panic(err)
	}
	if testing.Verbose() {
		serv.DebugLogger = log.New(os.Stderr, "filedrop/debug ", log.Lshortfile)
	}
	return serv
}

func cleanServ(serv *filedrop.Server) {
	if _, err := serv.DB.Exec(`DROP TABLE filedrop`); err != nil {
		panic(err)
	}
	serv.Close()
	os.Remove(serv.Conf.StorageDir)
}

func doPOST(t *testing.T, c *http.Client, url string, contentType string, reqBody io.Reader) []byte {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
