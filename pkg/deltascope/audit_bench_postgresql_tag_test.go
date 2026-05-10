//go:build postgresql

// Package deltascope benchmarks the public audit API on representative PostgreSQL SQL inputs.
// input: deterministic PostgreSQL public audit requests without metadata providers
// output: benchmark timing and allocation measurements for public API cache evaluation
// pos: PostgreSQL-tagged public API benchmark coverage for parser/cache evaluation
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

const benchPGPublicSimpleDDL = "CREATE TABLE users (id SERIAL PRIMARY KEY, name varchar(100), email varchar(255) UNIQUE)"

const benchPGPublicMixed = `CREATE TABLE users (id SERIAL PRIMARY KEY, name varchar(100) NOT NULL, email varchar(255) UNIQUE);
ALTER TABLE users ADD COLUMN age integer DEFAULT 0;
INSERT INTO users (id, name, email) VALUES (1, 'test', 'test@example.com');
DELETE FROM users WHERE id = 1`

func BenchmarkAuditPublicPGSimpleDDL(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     benchPGPublicSimpleDDL,
			Dialect: DialectPostgreSQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditPublicPGMixed(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     benchPGPublicMixed,
			Dialect: DialectPostgreSQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
