package filedrop

import "database/sql"

type db struct {
	*sql.DB
}

func openDB(driver, dsn string) (*db, error) {
	return &db{}, nil
}
