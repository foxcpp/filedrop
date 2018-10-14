package filedrop

import (
	"database/sql"
	"strings"
	"time"
)

type db struct {
	*sql.DB

	addFile     *sql.Stmt
	remFile     *sql.Stmt
	contentType *sql.Stmt

	addUse         *sql.Stmt
	shouldDelete   *sql.Stmt
	cleanup        *sql.Stmt
	pendingCleanup *sql.Stmt
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

	if driver == "sqlite3" {
		// Also some optimizations for SQLite to make it FAA-A-A-AST.
		db.Exec(`PRAGMA foreign_keys = ON`)
		db.Exec(`PRAGMA auto_vacuum = INCREMENTAL`)
		db.Exec(`PRAGMA journal_mode = WAL`)
		db.Exec(`PRAGMA defer_foreign_keys = ON`)
		db.Exec(`PRAGMA synchronous = NORMAL`)
		db.Exec(`PRAGMA temp_store = MEMORY`)
		db.Exec(`PRAGMA cache_size = 5000`)
	}

	if err := db.initSchema(); err != nil {
		panic(err)
	}
	if err := db.initStmts(); err != nil {
		panic(err)
	}
	return db, nil
}

func (db *db) initSchema() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS filedrop (
		uuid TEXT PRIMARY KEY NOT NULL,
		contentType TEXT DEFAULT NULL,
		uses INTEGER NOT NULL DEFAULT 0,
		maxUses INTEGER DEFAULT NULL,
		storeUntil INTEGER DEFAULT NULL
	)`)
	if err != nil {
		return err
	}
	return nil
}

func (db *db) initStmts() error {
	var err error
	db.addFile, err = db.Prepare(`INSERT INTO filedrop(uuid, contentType, maxUses, storeUntil) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	db.remFile, err = db.Prepare(`DELETE FROM filedrop WHERE uuid = ?`)
	if err != nil {
		return err
	}
	db.contentType, err = db.Prepare(`SELECT contentType FROM filedrop WHERE uuid = ?`)
	if err != nil {
		return err
	}
	db.shouldDelete, err = db.Prepare(`SELECT exists(SELECT uuid FROM filedrop WHERE uuid = ? AND (storeUntil < ? OR maxUses == uses))`)
	if err != nil {
		return err
	}
	db.addUse, err = db.Prepare(`UPDATE filedrop SET uses = (SELECT uses+1 FROM filedrop WHERE uuid = ?) WHERE uuid = ?`)
	if err != nil {
		return err
	}
	db.pendingCleanup, err = db.Prepare(`SELECT uuid FROM filedrop WHERE storeUntil < ? OR maxUses == uses`)
	if err != nil {
		return err
	}
	db.cleanup, err = db.Prepare(`DELETE FROM filedrop WHERE storeUntil < ? OR maxUses == uses`)
	if err != nil {
		return err
	}
	return nil
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
	res := 0
	if err := row.Scan(&res); err != nil {
		return false
	}
	return res == 1
}

func (db *db) AddUse(tx *sql.Tx, uuid string) error {
	if tx != nil {
		_, err := tx.Stmt(db.addUse).Exec(uuid, uuid)
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

func (db *db) UnreachableFiles(tx *sql.Tx) ([]string, error) {
	uuids := []string{}
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = tx.Stmt(db.pendingCleanup).Query(time.Now().Unix())
	} else {
		rows, err = db.pendingCleanup.Query(time.Now().Unix())
	}
	if err != nil {
		return uuids, err
	}
	for rows.Next() {
		uuid := ""
		if err := rows.Scan(&uuid); err != nil {
			return uuids, err
		}
	}
	return uuids, nil
}

func (db *db) RemoveUnreachableFiles(tx *sql.Tx) error {
	if tx != nil {
		_, err := tx.Stmt(db.cleanup).Exec(time.Now().Unix())
		return err
	} else {
		_, err := db.cleanup.Exec(time.Now().Unix())
		return err
	}
}
