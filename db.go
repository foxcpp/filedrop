package filedrop

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

type db struct {
	*sql.DB

	Driver, DSN string

	addFile     *sql.Stmt
	remFile     *sql.Stmt
	contentType *sql.Stmt

	addUse           *sql.Stmt
	shouldDelete     *sql.Stmt
	removeStaleFiles *sql.Stmt
	staleFiles       *sql.Stmt
}

func openDB(driver, dsn string) (*db, error) {
	if driver == "sqlite3" {
		// We apply some tricks for SQLite to avoid "database is locked" errors.

		if !strings.HasPrefix(dsn, "file:") {
			dsn = "file:" + dsn
		}
		if !strings.Contains(dsn, "?") {
			dsn = dsn + "?"
		}
		dsn = dsn + "cache=shared&_journal=WAL&_busy_timeout=5000"
	}

	db := new(db)
	var err error
	db.DB, err = sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.Driver = driver
	db.DSN = dsn

	if driver == "mysql" {
		db.Exec(`SET SESSION TRANSACTION ISOLATION LEVEL SERIALIZABLE`)
	}
	if driver == "sqlite3" {
		db.DB.SetMaxOpenConns(1)
		// Also some optimizations for SQLite to make it FAA-A-A-AST.
		db.Exec(`PRAGMA auto_vacuum = INCREMENTAL`)
		db.Exec(`PRAGMA journal_mode = WAL`)
		db.Exec(`PRAGMA synchronous = NORMAL`)
		db.Exec(`PRAGMA cache_size = 5000`)
	}

	db.initSchema()
	db.initStmts()
	return db, nil
}

func (db *db) initSchema() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS filedrop (
		uuid CHAR(36) PRIMARY KEY NOT NULL,
		contentType VARCHAR(255) DEFAULT NULL,
		uses INTEGER NOT NULL DEFAULT 0,
		maxUses INTEGER DEFAULT NULL,
		storeUntil BIGINT DEFAULT NULL
	)`)
	if err != nil {
		panic(err)
	}
}

func (db *db) reformatBindvars(raw string) (res string) {
	// THIS IS VERY LIMITED IMPLEMENTATION.
	// If someday this will become not enough - just switch to https://github.com/jmoiron/sqlx.
	res = raw

	// sqlite3 supports both $N and ?.
	// mysql supports only ?.
	// postgresql supports only $1 (SHOWFLAKE!!!).

	if db.Driver == "postgres" {
		varCount := strings.Count(raw, "?")
		for i := 1; i <= varCount; i++ {
			res = strings.Replace(res, "?", "$"+strconv.Itoa(i), 1)
		}
	}
	return
}

func (db *db) Prepare(query string) (*sql.Stmt, error) {
	return db.DB.Prepare(db.reformatBindvars(query))
}

func (db *db) initStmts() {
	var err error
	db.addFile, err = db.Prepare(`INSERT INTO filedrop(uuid, contentType, maxUses, storeUntil) VALUES (?, ?, ?, ?)`)
	if err != nil {
		panic(err)
	}
	db.remFile, err = db.Prepare(`DELETE FROM filedrop WHERE uuid = ?`)
	if err != nil {
		panic(err)
	}
	db.contentType, err = db.Prepare(`SELECT contentType FROM filedrop WHERE uuid = ?`)
	if err != nil {
		panic(err)
	}
	db.shouldDelete, err = db.Prepare(`SELECT EXISTS(SELECT uuid FROM filedrop WHERE uuid = ? AND (storeUntil < ? OR maxUses = uses))`)
	if err != nil {
		panic(err)
	}
	db.addUse, err = db.Prepare(`UPDATE filedrop SET uses = uses + 1 WHERE uuid = ?`)
	if err != nil {
		panic(err)
	}
	db.staleFiles, err = db.Prepare(`SELECT uuid FROM filedrop WHERE storeUntil < ? OR maxUses = uses`)
	if err != nil {
		panic(err)
	}
	db.removeStaleFiles, err = db.Prepare(`DELETE FROM filedrop WHERE storeUntil < ? OR maxUses = uses`)
	if err != nil {
		panic(err)
	}
}

func (db *db) AddFile(tx *sql.Tx, uuid string, contentType string, maxUses uint, storeUntil time.Time) error {
	maxUsesN := sql.NullInt64{Int64: int64(maxUses), Valid: maxUses != 0}
	storeUntilN := sql.NullInt64{Int64: storeUntil.Unix(), Valid: !storeUntil.IsZero()}
	contentTypeN := sql.NullString{String: contentType, Valid: contentType != ""}

	if tx != nil {
		_, err := tx.Stmt(db.addFile).Exec(uuid, contentTypeN, maxUsesN, storeUntilN)
		return err
	} else {
		_, err := db.addFile.Exec(uuid, contentTypeN, maxUsesN, storeUntilN)
		return err
	}
}

func (db *db) RemoveFile(tx *sql.Tx, uuid string) error {
	if tx != nil {
		_, err := tx.Stmt(db.remFile).Exec(uuid)
		return err
	} else {
		_, err := db.remFile.Exec(uuid)
		return err
	}
}

func (db *db) ShouldDelete(tx *sql.Tx, uuid string) bool {
	var row *sql.Row
	if tx != nil {
		row = tx.Stmt(db.shouldDelete).QueryRow(uuid, time.Now().Unix())
	} else {
		row = db.shouldDelete.QueryRow(uuid, time.Now().Unix())
	}
	if db.Driver != "postgres" {
		res := 0
		if err := row.Scan(&res); err != nil {
			return false
		}
		return res == 1
	} else {
		res := false
		if err := row.Scan(&res); err != nil {
			return false
		}
		return res
	}
}

func (db *db) AddUse(tx *sql.Tx, uuid string) error {
	if tx != nil {
		_, err := tx.Stmt(db.addUse).Exec(uuid)
		return err
	} else {
		_, err := db.addUse.Exec(uuid, uuid)
		return err
	}
}

func (db *db) ContentType(tx *sql.Tx, fileUUID string) (string, error) {
	var row *sql.Row
	if tx != nil {
		row = tx.Stmt(db.contentType).QueryRow(fileUUID)
	} else {
		row = db.contentType.QueryRow(fileUUID)
	}

	res := sql.NullString{}
	return res.String, row.Scan(&res)
}

func (db *db) StaleFiles(tx *sql.Tx, now time.Time) ([]string, error) {
	uuids := []string{}
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = tx.Stmt(db.staleFiles).Query(now.Unix())
	} else {
		rows, err = db.staleFiles.Query(now.Unix())
	}
	if err != nil {
		return uuids, err
	}
	for rows.Next() {
		uuid := ""
		if err := rows.Scan(&uuid); err != nil {
			return uuids, err
		}
		uuids = append(uuids, uuid)
	}
	return uuids, nil
}

func (db *db) RemoveStaleFiles(tx *sql.Tx, now time.Time) error {
	if tx != nil {
		_, err := tx.Stmt(db.removeStaleFiles).Exec(now.Unix())
		return err
	} else {
		_, err := db.removeStaleFiles.Exec(now.Unix())
		return err
	}
}
