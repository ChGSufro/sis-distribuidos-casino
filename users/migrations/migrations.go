// Package migrations embebe las migraciones SQL de users_db.
package migrations

import _ "embed"

// Init contiene el esquema inicial (0001_init.sql).
//
//go:embed 0001_init.sql
var Init string
