package filedrop

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

var ErrFileDoesntExists = errors.New("file doesn't exists")

// Main filedrop server structure, implements http.Handler.
type Server struct {
	DB          *db
	Conf        Config
	Logger      *log.Logger
	DebugLogger *log.Logger

	fileCleanerStopChan chan bool
}

// Create and initialize new server instance using passed configuration.
//
// serv.Logger will be redirected to os.Stderr by default.
// Created instances should be closed by using serv.Close.
func New(conf Config) (*Server, error) {
	s := new(Server)
	var err error

	s.Conf = conf

	if err := os.MkdirAll(conf.StorageDir, os.ModePerm); err != nil {
		return nil, err
	}
	if err := s.testPerms(); err != nil {
		return nil, err
	}

	s.fileCleanerStopChan = make(chan bool)
	s.Logger = log.New(os.Stderr, "filedrop ", log.LstdFlags)
	s.DB, err = openDB(conf.DB.Driver, conf.DB.DSN)

	go s.fileCleaner()

	return s, err
}

func (s *Server) dbgLog(v ...interface{}) {
	if s.DebugLogger != nil {
		s.DebugLogger.Output(2, fmt.Sprintln(v...))
	}
}

func (s *Server) testPerms() error {
	testPath := filepath.Join(s.Conf.StorageDir, "test_file")

	// Check write permissions.
	f, err := os.Create(testPath)
	if err != nil {
		return err
	}
	f.Close()

	// Check read permissions.
	f, err = os.Open(testPath)
	if err != nil {
		return err
	}
	f.Close()

	// Check remove permissions.
	return os.Remove(testPath)
}

// AddFile adds file to storage and returns assigned UUID which can be directly
// substituted into URL.
func (s *Server) AddFile(contents io.Reader, contentType string, maxUses uint, storeUntil time.Time) (string, error) {
	fileUUID, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrap(err, "UUID generation")
	}
	outLocation := filepath.Join(s.Conf.StorageDir, fileUUID.String())

	_, err = os.Stat(outLocation)
	if err == nil {
		s.Logger.Println("UUID collision detected:", fileUUID)
		return "", errors.New("UUID collision detected")
	}

	file, err := os.Create(outLocation)
	if err != nil {
		s.Logger.Printf("File create failure (%v): %v\n", fileUUID, err)
		return "", errors.Wrap(err, "file open")
	}
	if _, err := io.Copy(file, contents); err != nil {
		s.Logger.Printf("File write failure (%v): %v\n", fileUUID, err)
		return "", errors.Wrap(err, "file write")
	}
	if err := s.DB.AddFile(nil, fileUUID.String(), contentType, maxUses, storeUntil); err != nil {
		os.Remove(outLocation)
		s.Logger.Printf("DB add failure (%v, %v, %v, %v): %v\n", fileUUID, contentType, maxUses, storeUntil, err)
		return "", errors.Wrap(err, "db add")
	}

	return fileUUID.String(), nil
}

// RemoveFile removes file from database and underlying storage.
func (s *Server) RemoveFile(fileUUID string) error {
	return s.removeFile(nil, fileUUID)
}

func (s *Server) removeFile(tx *sql.Tx, fileUUID string) error {
	fileLocation := filepath.Join(s.Conf.StorageDir, fileUUID)

	// Just to check validity.
	_, err := uuid.FromString(fileUUID)
	if err != nil {
		return errors.Wrap(err, "uuid parse")
	}

	if err := s.DB.RemoveFile(tx, fileUUID); err != nil {
		s.Logger.Printf("DB remove failure (%v): %v\n", fileUUID, err)
		return errors.Wrap(err, "db remove")
	}

	if err := os.Remove(fileLocation); err != nil {
		// TODO: Recover DB entry?
		s.Logger.Printf("File remove failure (%v): %v\n", fileUUID, err)
		return errors.Wrap(err, "file remove")
	}
	return nil
}

// OpenFile opens file for reading without any other side-effects
// applied (such as "link" usage counting).
func (s *Server) OpenFile(fileUUID string) (io.ReadSeeker, error) {
	// Just to check validity.
	_, err := uuid.FromString(fileUUID)
	if err != nil {
		return nil, errors.Wrap(err, "uuid parse")
	}

	fileLocation := filepath.Join(s.Conf.StorageDir, fileUUID)
	file, err := os.Open(fileLocation)
	if err != nil {
		if os.IsNotExist(err) {
			s.removeFile(nil, fileUUID)
			return nil, ErrFileDoesntExists
		}
		return nil, err
	}
	return file, nil
}

// GetFile opens file for reading.
//
// Note that access using this function is equivalent to access
// through HTTP API, so it will count against usage count, for example.
// To avoid this use OpenFile(fileUUID).
func (s *Server) GetFile(fileUUID string) (r io.ReadSeeker, contentType string, err error) {
	// Just to check validity.
	_, err = uuid.FromString(fileUUID)
	if err != nil {
		return nil, "", ErrFileDoesntExists
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, "", errors.Wrap(err, "tx begin")
	}
	defer tx.Rollback() // rollback is no-op after commit

	s.dbgLog("Serving file", fileUUID)

	if s.DB.ShouldDelete(tx, fileUUID) {
		s.dbgLog("File removed just before getting, UUID:", fileUUID)
		if err := s.removeFile(tx, fileUUID); err != nil {
			s.Logger.Println("Error while trying to remove file", fileUUID+":", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, "", err
		}
		return nil, "", ErrFileDoesntExists
	}
	if err := s.DB.AddUse(tx, fileUUID); err != nil {
		return nil, "", errors.Wrap(err, "add use")
	}

	fileLocation := filepath.Join(s.Conf.StorageDir, fileUUID)
	r, err = os.Open(fileLocation)
	if err != nil {
		if os.IsNotExist(err) {
			s.DB.RemoveFile(tx, fileUUID)
			return nil, "", ErrFileDoesntExists
		}
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", errors.Wrap(err, "tx commit")
	}

	ttype, err := s.DB.ContentType(nil, fileUUID)
	if err != nil {
		return nil, "", errors.Wrap(err, "content type query")
	}

	return r, ttype, nil
}

func (s *Server) acceptFile(w http.ResponseWriter, r *http.Request) {
	splittenPath := strings.Split(r.URL.Path, "/")

	if s.Conf.UploadAuth.Callback != nil && !s.Conf.UploadAuth.Callback(r) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 forbidden"))
		return
	}

	if s.Conf.Limits.MaxFileSize != 0 && r.ContentLength > int64(s.Conf.Limits.MaxFileSize) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("413 request entity too large"))
		return
	}

	storeUntil := time.Time{}
	if r.URL.Query().Get("store-secs") == "" && s.Conf.Limits.MaxStoreSecs != 0 {
		storeUntil = time.Now().Add(time.Duration(s.Conf.Limits.MaxStoreSecs) * time.Second)
	} else if r.URL.Query().Get("store-secs") != "" {
		secs, err := strconv.Atoi(r.URL.Query().Get("store-secs"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 bad request (invalid store-secs value)"))
			return
		}
		if s.Conf.Limits.MaxStoreSecs != 0 && uint(secs) > s.Conf.Limits.MaxStoreSecs {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 bad request (too big store-secs value)"))
			return
		}
		storeUntil = time.Now().Add(time.Duration(secs) * time.Second)
	}
	var maxUses uint
	if r.URL.Query().Get("max-uses") == "" && s.Conf.Limits.MaxUses != 0 {
		maxUses = s.Conf.Limits.MaxUses
	} else if r.URL.Query().Get("max-uses") != "" {
		var err error
		maxUses, err := strconv.Atoi(r.URL.Query().Get("max-uses"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 bad request (invalid max-uses value)"))
			return
		}
		if s.Conf.Limits.MaxUses != 0 && uint(maxUses) > s.Conf.Limits.MaxUses {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 bad request (too big max-uses value)"))
			return
		}
	}

	fileUUID, err := s.AddFile(r.Body, r.Header.Get("Content-Type"), maxUses, storeUntil)
	if err != nil {
		s.Logger.Println("Error while serving", r.RequestURI+":", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	s.dbgLog("Accepted file, assigned UUID is", fileUUID)

	// Smart logic to convert request's URL into absolute result URL.
	resURL := url.URL{}
	if r.Header.Get("X-HTTPS-Downstream") == "1" {
		resURL.Scheme = "https"
	} else if r.Header.Get("X-HTTPS-Downstream") == "0" {
		resURL.Scheme = "http"
	} else if s.Conf.HTTPSDownstream {
		resURL.Scheme = "https"
	} else {
		resURL.Scheme = "http"
	}
	resURL.Host = r.Host
	splittenPath = append(splittenPath, fileUUID)
	resURL.Path = strings.Join(splittenPath, "/")

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(resURL.String()))
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	if s.Conf.DownloadAuth.Callback != nil && !s.Conf.DownloadAuth.Callback(r) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 forbidden"))
		return
	}

	splittenPath := strings.Split(r.URL.Path, "/")
	if len(splittenPath) < 2 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 not found"))
		return
	}
	fileUUID := splittenPath[len(splittenPath)-1]
	if _, err := uuid.FromString(fileUUID); err != nil {
		// Probably last component is fake "filename".
		if len(splittenPath) == 1 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 not found"))
			return
		}
		fileUUID = splittenPath[len(splittenPath)-2]
	}
	reader, ttype, err := s.GetFile(fileUUID)
	if err != nil {
		if err == ErrFileDoesntExists {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 not found"))
		} else {
			s.Logger.Println("Error while serving", r.RequestURI+":", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
		return
	}
	if ttype != "" {
		w.Header().Set("Content-Type", ttype)
	}
	w.Header().Set("ETag", fileUUID)
	w.Header().Set("Cache-Control", "public, immutable, max-age=31536000")
	http.ServeContent(w, r, fileUUID, time.Time{}, reader)
}

// ServeHTTP implements http.Handler for filedrop.Server.
//
// Note that filedrop code is URL prefix-agnostic, so request URI doesn't
// matters much.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.acceptFile(w, r)
	} else if r.Method == http.MethodGet || r.Method == http.MethodHead {
		s.serveFile(w, r)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("405 method not allowed"))
	}
}

func (s *Server) Close() error {
	// don't close DB if "cleaner" is doing something, wait for it to finish
	s.fileCleanerStopChan <- true
	<-s.fileCleanerStopChan

	return s.DB.Close()
}

func (s *Server) fileCleaner() {
	if s.Conf.CleanupIntervalSecs == 0 {
		s.Conf.CleanupIntervalSecs = 60
	}
	tick := time.NewTicker(time.Duration(s.Conf.CleanupIntervalSecs) * time.Second)
	for {
		select {
		case <-s.fileCleanerStopChan:
			s.fileCleanerStopChan <- true
			return
		case <-tick.C:
			s.cleanupFiles()
		}
	}
}

func (s *Server) cleanupFiles() {
	tx, err := s.DB.Begin()
	if err != nil {
		s.Logger.Println("Failed to begin transaction for clean-up:", err)
		return
	}
	defer tx.Rollback() // rollback is no-op after commit

	now := time.Now()

	uuids, err := s.DB.StaleFiles(tx, now)
	if err != nil {
		s.Logger.Println("Failed to get list of files pending removal:", err)
		return
	}

	if len(uuids) != 0 {
		s.dbgLog(len(uuids), "file to be removed")
	}

	for _, fileUUID := range uuids {
		if err := os.Remove(filepath.Join(s.Conf.StorageDir, fileUUID)); err != nil {
			s.Logger.Println("Failed to remove file during clean-up:", err)
		}
	}

	if err := s.DB.RemoveStaleFiles(tx, now); err != nil {
		s.Logger.Println("Failed to remove stale files from DB:", err)
		return
	}

	if err := tx.Commit(); err != nil {
		s.Logger.Println("Failed to begin transaction for clean-up:", err)
	}
}
