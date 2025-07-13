package graph

import "database/sql"

// Dependency injection for the app.
type Resolver struct {
	DB          *sql.DB
	WalletTable string // name of DB table
}
