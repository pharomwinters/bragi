package database

import (
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// driverName is the sql.Open driver name. sqlite-vec is registered globally
// via sqlite3_auto_extension, so we use the standard mattn/go-sqlite3 driver.
const driverName = "sqlite3"

func init() {
	sqlite_vec.Auto()
}
