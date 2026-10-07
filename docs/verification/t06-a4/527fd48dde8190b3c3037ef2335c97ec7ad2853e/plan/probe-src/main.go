// tmp-a4probe: T06-A4-PLAN read-only provider probe.
// Calls the real exported Provider.LoadTableSnapshot against the disposable
// mysql84 fixture; prints the snapshot's column default-identity fields as
// produced by production code. No reimplementation of loadColumns.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"

	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

func main() {
	dsn := os.Getenv("PROBE_DSN")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(2)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "ping:", err)
		os.Exit(2)
	}
	p := mysqlmeta.NewProvider(db)
	snap, err := p.LoadTableSnapshot(context.Background(), "", os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(2)
	}
}
