package filedrop

import (
	"bytes"
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
	"sync/atomic"
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

	fileCleanerRunTime int64
}

// Create and initialize new server instance using passed configuration.
//
// serv.Logger will be redirected to os.Stderr by default.
// Created instances should be closed by using serv.Close.
func New(conf Config) (*Server, error) {
	s := new(Server)
	var err error

	s.Conf = conf
	if s.Conf.CleanupIntervalSecs < 1 {
		s.Conf.CleanupIntervalSecs = 60
	}

	if err := os.MkdirAll(conf.StorageDir, os.ModePerm); err != nil {
		return nil, err
	}
	if err := s.testPerms(); err != nil {
		return nil, err
	}
	
	s.Logger = log.New(os.Stderr, "filedrop ", log.LstdFlags)
	s.DB, err = openDB(conf.DB.Driver, conf.DB.DSN)
	
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
			// Clean up the DB entry if the file was removed by an external program.
			if err := s.DB.RemoveFile(nil, fileUUID); err != nil {
				s.Logger.Printf("DB remove failure (%v): %v\n", fileUUID, err)
			}
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
			return nil, "", ErrFileDoesntExists
		}
		// Clean up the DB entry if the file was removed by an external program.
		if err := s.DB.RemoveFile(tx, fileUUID); err != nil {
			s.Logger.Printf("DB remove failure (%v): %v\n", fileUUID, err)
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
	if s.Conf.UploadAuth.Callback != nil && !s.Conf.UploadAuth.Callback(r) {
		s.Logger.Printf("Authentication failure (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
		s.writeErr(w, r, http.StatusForbidden, "forbidden")
		return
	}

	if s.Conf.Limits.MaxFileSize != 0 && r.ContentLength > int64(s.Conf.Limits.MaxFileSize) {
		s.Logger.Printf("Too big file (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
		s.writeErr(w, r, http.StatusRequestEntityTooLarge, "too big file")
		return
	}

	storeUntil := time.Time{}
	if r.URL.Query().Get("store-secs") == "" && s.Conf.Limits.MaxStoreSecs != 0 {
		storeUntil = time.Now().Add(time.Duration(s.Conf.Limits.MaxStoreSecs) * time.Second)
	} else if r.URL.Query().Get("store-secs") != "" {
		secs, err := strconv.Atoi(r.URL.Query().Get("store-secs"))
		if err != nil {
			s.Logger.Printf("Invalid store-secs (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
			s.writeErr(w, r, http.StatusBadRequest, "invalid store-secs value")
			return
		}
		if s.Conf.Limits.MaxStoreSecs != 0 && uint(secs) > s.Conf.Limits.MaxStoreSecs {
			s.Logger.Printf("Too big store-secs (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
			s.writeErr(w, r, http.StatusBadRequest, "too big store-secs value")
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
			s.Logger.Printf("Invalid max-uses store-secs (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
			s.writeErr(w, r, http.StatusBadRequest, "invalid max-uses value")
			return
		}
		if s.Conf.Limits.MaxUses != 0 && uint(maxUses) > s.Conf.Limits.MaxUses {
			s.Logger.Printf("Too big max-uses store-secs (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
			s.writeErr(w, r, http.StatusBadRequest, "too big max-uses value")
			return
		}
	}

	fileUUID, err := s.AddFile(r.Body, r.Header.Get("Content-Type"), maxUses, storeUntil)
	if err != nil {
		s.Logger.Println("Error while serving", r.RequestURI+":", err)
		s.writeErr(w, r, http.StatusInternalServerError, "internal server error")
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
	splittenPath := strings.Split(r.URL.Path, "/")
	if r.URL.Path == "/" {
		splittenPath = nil
	}
	splittenPath = append(splittenPath, fileUUID)
	resURL.Path = strings.Join(splittenPath, "/")

	w.Header().Add("Content-Type", `text/plain; charset="us-ascii"`)
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte(resURL.String())); err != nil {
		s.Logger.Printf("I/O error (URL %v, IP %v): %v", r.URL.String(), r.RemoteAddr, err)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, r *http.Request, code int, replyText string) {
	w.Header().Add("Content-Type", `text/plain; charset="us-ascii"`)
	w.WriteHeader(code)
	_, err := io.WriteString(w, strconv.Itoa(code)+" "+replyText)
	if err != nil {
		s.Logger.Printf("I/O error (URL %v, IP %v): %v", r.URL.String(), r.RemoteAddr, err)
	}
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	if s.Conf.DownloadAuth.Callback != nil && !s.Conf.DownloadAuth.Callback(r) {
		s.Logger.Printf("Authentication failure (URL %v, IP %v)", r.URL.String(), r.RemoteAddr)
		s.writeErr(w, r, http.StatusForbidden, "forbidden")
		return
	}

	splittenPath := strings.Split(r.URL.Path, "/")
	if len(splittenPath) < 2 {
		s.writeErr(w, r, http.StatusNotFound, "not found")
		return
	}
	fileUUID := splittenPath[len(splittenPath)-1]
	if _, err := uuid.FromString(fileUUID); err != nil {
		// Probably last component is fake "filename".
		if len(splittenPath) == 1 {
			s.writeErr(w, r, http.StatusNotFound, "not found")
			return
		}
		fileUUID = splittenPath[len(splittenPath)-2]
	}
	reader, ttype, err := s.GetFile(fileUUID)
	if err != nil {
		if err == ErrFileDoesntExists {
			s.writeErr(w, r, http.StatusNotFound, "not found")
		} else {
			s.Logger.Println("Error while serving", r.RequestURI+":", err)
			s.writeErr(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if ttype != "" {
		w.Header().Set("Content-Type", ttype)
	}
	w.Header().Set("ETag", fileUUID)
	w.Header().Set("Cache-Control", "public, immutable, max-age=31536000")
	if r.Method == http.MethodOptions {
		reader = bytes.NewReader([]byte{})
	}
	http.ServeContent(w, r, fileUUID, time.Time{}, reader)
}

// ServeHTTP implements http.Handler for filedrop.Server.
//
// Note that filedrop code is URL prefix-agnostic, so request URI doesn't
// matters much.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", s.Conf.AllowedOrigins)
	if r.Method == http.MethodPost {
		s.acceptFile(w, r)
	} else if r.Method == http.MethodGet || r.Method == http.MethodHead {
		s.serveFile(w, r)
	} else if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length")
		w.WriteHeader(http.StatusNoContent)
	} else {
		s.writeErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}

	// Run a cleanup in a goroutine only if last cleanup was more than CleanupIntervalSecs seconds ago.
	if time.Now().Unix() - atomic.LoadInt64(&s.fileCleanerRunTime) > int64(s.Conf.CleanupIntervalSecs) {
		// Set run time twice to exclude cleanup time.
		atomic.StoreInt64(&s.fileCleanerRunTime, time.Now().Unix())
		s.cleanupFiles()
		atomic.StoreInt64(&s.fileCleanerRunTime, time.Now().Unix())
	}
}

func (s *Server) Close() error {
	return s.DB.Close()
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
