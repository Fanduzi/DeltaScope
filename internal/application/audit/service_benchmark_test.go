package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// simpleSQL is a basic CREATE TABLE for benchmarking single-statement audit.
const simpleSQL = "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100), email VARCHAR(255) UNIQUE)"

// complexSQL contains multiple statements with various features to stress the evaluation loop.
const complexSQL = `CREATE TABLE users (
	id INT PRIMARY KEY,
	name VARCHAR(100) NOT NULL,
	email VARCHAR(255) UNIQUE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	INDEX idx_name (name),
	INDEX idx_email (email)
);
CREATE TABLE orders (
	id INT PRIMARY KEY,
	user_id INT,
	amount DECIMAL(10,2),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id),
	INDEX idx_user (user_id),
	INDEX idx_amount (amount)
);
ALTER TABLE users ADD COLUMN age INT DEFAULT 0;
INSERT INTO users (id, name, email) VALUES (1, 'test', 'test@example.com');
`

// multiStatementSQL contains 10 CREATE TABLE statements for throughput benchmarking.
const multiStatementSQL = `CREATE TABLE t1 (id INT PRIMARY KEY, c1 VARCHAR(100));
CREATE TABLE t2 (id INT PRIMARY KEY, c1 VARCHAR(100), c2 INT DEFAULT 0);
CREATE TABLE t3 (id INT PRIMARY KEY, c1 VARCHAR(100) NOT NULL, c2 INT, c3 TEXT);
CREATE TABLE t4 (id INT PRIMARY KEY, c1 VARCHAR(255) UNIQUE, c2 DECIMAL(10,2));
CREATE TABLE t5 (id INT PRIMARY KEY, c1 INT, c2 VARCHAR(100), INDEX idx_c1 (c1));
CREATE TABLE t6 (id INT PRIMARY KEY, c1 VARCHAR(100), c2 TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE t7 (id INT PRIMARY KEY, c1 VARCHAR(100), c2 VARCHAR(255), c3 INT, UNIQUE KEY uk_c2 (c2));
CREATE TABLE t8 (id INT PRIMARY KEY, c1 VARCHAR(100), FOREIGN KEY (c1) REFERENCES t1(c1));
CREATE TABLE t9 (id INT PRIMARY KEY, c1 VARCHAR(100) NOT NULL DEFAULT 'default', c2 INT CHECK (c2 > 0));
CREATE TABLE t10 (id INT PRIMARY KEY, c1 VARCHAR(100), c2 VARCHAR(255), c3 DECIMAL(10,2), INDEX idx_c1_c2 (c1, c2));
`

func BenchmarkAuditSimpleSQL(b *testing.B) {
	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.Audit(ctx, Request{
			SQL:     simpleSQL,
			Dialect: spec.DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditComplexSQL(b *testing.B) {
	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.Audit(ctx, Request{
			SQL:     complexSQL,
			Dialect: spec.DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditMultiStatement(b *testing.B) {
	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.Audit(ctx, Request{
			SQL:     multiStatementSQL,
			Dialect: spec.DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAuditSQLTiDB tests the TiDB dialect path.
func BenchmarkAuditSQLTiDB(b *testing.B) {
	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.Audit(ctx, Request{
			SQL:     complexSQL,
			Dialect: spec.DialectTiDB,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
