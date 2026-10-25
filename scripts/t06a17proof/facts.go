// Package main fixes the independent T06-A17-R1 24-column proof contract.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

const createSQL = `CREATE TABLE golden.t06_a17_type_facts (
  d2 DECIMAL(12,2),
  d3 DECIMAL(12,3),
  d0 DECIMAL(12,0),
  dm DECIMAL(12),
  dn DECIMAL,
  f2 FLOAT(10,2),
  f3 FLOAT(10,3),
  f0 FLOAT(10,0),
  fn FLOAT,
  x2 DOUBLE(16,2),
  x4 DOUBLE(16,4),
  x0 DOUBLE(16,0),
  xn DOUBLE,
  e_upper ENUM('Ready','Closed'),
  e_lower ENUM('ready','closed'),
  e_order ENUM('Closed','Ready'),
  s_upper SET('Read','Write'),
  s_lower SET('read','write'),
  s_order SET('Write','Read'),
  e_escaped ENUM('','A,B','A)B','O''Reilly'),
  s_escaped SET('Read','A)B','O''Reilly'),
  i INT,
  v VARCHAR(8),
  u BIGINT UNSIGNED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`

var columnNames = []string{
	"d2", "d3", "d0", "dm", "dn", "f2", "f3", "f0", "fn",
	"x2", "x4", "x0", "xn", "e_upper", "e_lower", "e_order",
	"s_upper", "s_lower", "s_order", "e_escaped", "s_escaped", "i", "v", "u",
}

var columnFamilies = []string{
	"decimal", "decimal", "decimal", "decimal", "decimal", "float", "float", "float", "float",
	"double", "double", "double", "double", "enum", "enum", "enum",
	"set", "set", "set", "enum", "set", "int", "varchar", "bigint",
}

var memberFacts = map[string][]string{
	"e_upper":   {"Ready", "Closed"},
	"e_lower":   {"ready", "closed"},
	"e_order":   {"Closed", "Ready"},
	"s_upper":   {"Read", "Write"},
	"s_lower":   {"read", "write"},
	"s_order":   {"Write", "Read"},
	"e_escaped": {"", "A,B", "A)B", "O'Reilly"},
	"s_escaped": {"Read", "A)B", "O'Reilly"},
}

type anchor struct {
	Name    string
	Port    string
	Version string
	Dialect spec.Dialect
}

func anchorFor(name string) (anchor, error) {
	anchors := map[string]anchor{
		"mysql57": {"mysql57", "23357", "5.7.44", spec.DialectMySQL},
		"mysql80": {"mysql80", "23380", "8.0.46", spec.DialectMySQL},
		"mysql84": {"mysql84", "23384", "8.4.10", spec.DialectMySQL},
		"tidb85":  {"tidb85", "24000", "8.0.11-TiDB-v8.5.0", spec.DialectTiDB},
	}
	value, ok := anchors[name]
	if !ok {
		return anchor{}, fmt.Errorf("unknown anchor %q; expected mysql57, mysql80, mysql84, or tidb85", name)
	}
	return value, nil
}

type catalogRow struct {
	ColumnName       string  `json:"column_name"`
	OrdinalPosition  int     `json:"ordinal_position"`
	DataType         string  `json:"data_type"`
	ColumnType       string  `json:"column_type"`
	ColumnTypeHex    string  `json:"column_type_hex"`
	NumericPrecision *int64  `json:"numeric_precision"`
	NumericScale     *int64  `json:"numeric_scale"`
	CharacterSet     *string `json:"character_set_name"`
	Collation        *string `json:"collation_name"`
}

type pair struct {
	Column   string `json:"column"`
	Catalog  string `json:"catalog_column_type"`
	Provider string `json:"provider_type"`
	Length   int    `json:"provider_length"`
	Unsigned bool   `json:"provider_unsigned"`
}

func validateCatalog(ctx context.Context, rows []catalogRow) error {
	if len(rows) != len(columnNames) {
		return fmt.Errorf("catalog column count=%d, want %d", len(rows), len(columnNames))
	}
	seen := make(map[string]bool, len(rows))
	for i, row := range rows {
		if seen[row.ColumnName] {
			return fmt.Errorf("duplicate catalog column %q", row.ColumnName)
		}
		seen[row.ColumnName] = true
		if row.ColumnName != columnNames[i] || row.OrdinalPosition != i+1 {
			return fmt.Errorf("catalog position %d: name=%q ordinal=%d, want %q/%d", i, row.ColumnName, row.OrdinalPosition, columnNames[i], i+1)
		}
		if row.DataType != columnFamilies[i] {
			return fmt.Errorf("catalog %s DATA_TYPE=%q, want %q", row.ColumnName, row.DataType, columnFamilies[i])
		}
		decoded, err := hex.DecodeString(row.ColumnTypeHex)
		if err != nil || !bytes.Equal(decoded, []byte(row.ColumnType)) {
			return fmt.Errorf("catalog %s COLUMN_TYPE/HEX mismatch", row.ColumnName)
		}
		if i < 3 {
			wantScale := []int64{2, 3, 0}[i]
			if row.NumericPrecision == nil || row.NumericScale == nil || *row.NumericPrecision != 12 || *row.NumericScale != wantScale {
				return fmt.Errorf("catalog %s requires effective precision=12 scale=%d", row.ColumnName, wantScale)
			}
		}
		if wantMembers, ok := memberFacts[row.ColumnName]; ok {
			parsed, err := tidbparser.New().Parse(ctx, "CREATE TABLE member_fact (c "+row.ColumnType+")")
			if err != nil {
				return fmt.Errorf("catalog %s member decoding: %w", row.ColumnName, err)
			}
			if len(parsed.Statements) != 1 {
				return fmt.Errorf("catalog %s member definition statement count=%d", row.ColumnName, len(parsed.Statements))
			}
			create, ok := parsed.Statements[0].(*ast.CreateTableStmt)
			if !ok || len(create.Cols) != 1 || create.Cols[0].Tp == nil {
				return fmt.Errorf("catalog %s member definition is not one typed column", row.ColumnName)
			}
			if got := create.Cols[0].Tp.GetElems(); !slices.Equal(got, wantMembers) {
				return fmt.Errorf("catalog %s members=%q, want %q", row.ColumnName, got, wantMembers)
			}
		}
		if _, members := memberFacts[row.ColumnName]; members || row.ColumnName == "v" {
			if row.CharacterSet == nil || row.Collation == nil || *row.CharacterSet != "utf8mb4" || *row.Collation != "utf8mb4_bin" {
				return fmt.Errorf("catalog %s requires utf8mb4/utf8mb4_bin", row.ColumnName)
			}
		}
	}
	return nil
}

func pairCatalogProvider(rows []catalogRow, snapshot *spec.TableSnapshot, target anchor) ([]pair, error) {
	if snapshot == nil || !snapshot.Exists || snapshot.Table == nil {
		return nil, fmt.Errorf("provider snapshot missing or not present")
	}
	if snapshot.Schema != "golden" || snapshot.Table.Name != "t06_a17_type_facts" {
		return nil, fmt.Errorf("provider table identity mismatch: %s/%s", snapshot.Schema, snapshot.Table.Name)
	}
	if len(rows) != len(columnNames) || len(snapshot.Columns) != len(columnNames) {
		return nil, fmt.Errorf("pairing column counts: catalog=%d provider=%d, want %d each", len(rows), len(snapshot.Columns), len(columnNames))
	}
	if snapshot.Options["engine"] != "InnoDB" || snapshot.Options["charset"] != "utf8mb4" || snapshot.Options["collation"] != "utf8mb4_bin" {
		return nil, fmt.Errorf("provider table engine/charset/collation mismatch")
	}
	if snapshot.PrimaryKey != nil || len(snapshot.Indexes) != 0 {
		return nil, fmt.Errorf("provider returned an undeclared primary key or index")
	}
	pairs := make([]pair, 0, len(columnNames))
	seen := make(map[string]bool, len(columnNames))
	for i, name := range columnNames {
		column := snapshot.Columns[i]
		if seen[column.Name] {
			return nil, fmt.Errorf("duplicate provider column %q", column.Name)
		}
		seen[column.Name] = true
		if rows[i].ColumnName != name || column.Name != name {
			return nil, fmt.Errorf("pairing position %d: catalog=%q provider=%q, want %q", i, rows[i].ColumnName, column.Name, name)
		}
		if i < 21 {
			if column.Type != rows[i].ColumnType || column.Unsigned {
				return nil, fmt.Errorf("provider %s Type=%q unsigned=%v, catalog=%q unsigned=false", name, column.Type, column.Unsigned, rows[i].ColumnType)
			}
		} else {
			wantLength := 0
			if name == "v" {
				wantLength = 8
			} else if target.Name == "mysql57" {
				if name == "i" {
					wantLength = 11
				} else {
					wantLength = 20
				}
			}
			if column.Type != columnFamilies[i] || column.Length != wantLength || column.Unsigned != (name == "u") {
				return nil, fmt.Errorf("provider control %s=%q/%d/%v, want %q/%d/%v", name, column.Type, column.Length, column.Unsigned, columnFamilies[i], wantLength, name == "u")
			}
		}
		pairs = append(pairs, pair{name, rows[i].ColumnType, column.Type, column.Length, column.Unsigned})
	}
	return pairs, nil
}

func validateSession(session map[string]string, target anchor) error {
	if session["version"] != target.Version {
		return fmt.Errorf("anchor %s VERSION()=%q, want %q", target.Name, session["version"], target.Version)
	}
	for _, name := range []string{"character_set_client", "character_set_connection", "character_set_results", "collation_connection"} {
		if strings.TrimSpace(session[name]) == "" {
			return fmt.Errorf("missing session fact %s", name)
		}
	}
	if _, ok := session["sql_mode"]; !ok {
		return fmt.Errorf("missing session sql_mode")
	}
	return nil
}
