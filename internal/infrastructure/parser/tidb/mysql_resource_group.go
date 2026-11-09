package tidbparser

// mysql_resource_group.go recognizes the MySQL CPU resource-group
// family — CREATE/ALTER/DROP/SET RESOURCE GROUP forms the locked TiDB
// grammar cannot produce — and fabricates adapter statements carrying
// typed declarations (issue #103 T24). The recognizer claims only the
// pure MySQL shape: every option must come from the MySQL vocabulary and
// every member must parse inside its own grammar. Anything ambiguous —
// TiDB RU members, IF guards, vendor-mixed tails — falls through to the
// base parser so failures keep their original diagnostics, never a
// silently claimed half-parse.

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
)

// mysqlResourceGroupStatement is the adapter-owned carrier for one MySQL
// resource-group statement. The embedded node is a fabricated
// ast.StmtNode — a real ast type with only shared members set — so
// classify and the coverage-boundary paths keep their existing shape.
// The typed declaration rides beside it; extraction binds it instead of
// the TiDB RU declaration.
type mysqlResourceGroupStatement struct {
	ast.StmtNode
	operation   spec.DDLOperation
	declaration *spec.MySQLResourceGroupDeclaration
	// setFor marks the bounded `SET RESOURCE GROUP g FOR <ids>` form —
	// recognized surface, unaudited binding semantics; it carries no DDL
	// declaration.
	setFor bool
}

// parseMySQLResourceGroup recognizes a whole-input MySQL resource-group
// statement and fabricates its carrier node. A false result means the
// input is not a pure MySQL-form statement and must take the ordinary
// grammar path — the recognizer never emits partial claims.
func parseMySQLResourceGroup(sqlText string) (ast.StmtNode, bool) {
	toks := tokenizeSQL(sqlText)
	if len(toks) == 0 {
		return nil, false
	}
	// A trailing `;` is statement delimiter, not content.
	if last := toks[len(toks)-1]; last.kind == tokPunct && last.text(sqlText) == ";" {
		toks = toks[:len(toks)-1]
	}
	// CREATE|ALTER|DROP|SET RESOURCE GROUP <name> ...
	if len(toks) < 4 ||
		!toks[1].wordIs(sqlText, "resource") ||
		!toks[2].wordIs(sqlText, "group") {
		return nil, false
	}
	name, ok := mysqlResourceGroupName(sqlText, toks[3])
	if !ok {
		return nil, false
	}
	rest := toks[4:]
	switch {
	case toks[0].wordIs(sqlText, "create"):
		declaration, ok := mysqlRGCreateOptions(sqlText, rest)
		if !ok {
			return nil, false
		}
		node := &ast.CreateResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
		node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
		declaration.Name = name
		return &mysqlResourceGroupStatement{
			StmtNode:    node,
			operation:   spec.DDLOperationCreateResourceGroup,
			declaration: declaration,
		}, true
	case toks[0].wordIs(sqlText, "alter"):
		declaration, ok := mysqlRGAlterOptions(sqlText, rest)
		if !ok {
			return nil, false
		}
		declaration.Name = name
		node := &ast.AlterResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
		node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
		return &mysqlResourceGroupStatement{
			StmtNode:    node,
			operation:   spec.DDLOperationAlterResourceGroup,
			declaration: declaration,
		}, true
	case toks[0].wordIs(sqlText, "drop"):
		// FORCE is the only member MySQL DROP adds over the shared
		// grammar — bare and IF EXISTS forms stay on the base path.
		if len(rest) != 1 || !rest[0].wordIs(sqlText, "force") {
			return nil, false
		}
		node := &ast.DropResourceGroupStmt{ResourceGroupName: ast.NewCIStr(name)}
		node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
		return &mysqlResourceGroupStatement{
			StmtNode:  node,
			operation: spec.DDLOperationDropResourceGroup,
			declaration: &spec.MySQLResourceGroupDeclaration{
				Name: name,
				Options: []spec.MySQLResourceGroupOption{
					{Kind: spec.MySQLResourceGroupOptionForce},
				},
			},
		}, true
	case toks[0].wordIs(sqlText, "set"):
		// `SET RESOURCE GROUP g FOR <id>[,<id>]` — recognized MySQL
		// surface whose binding semantics stay unaudited; it carries no
		// declaration. The bare `SET RESOURCE GROUP g` current-thread
		// form parses in the base grammar and stays on that path.
		if len(rest) < 2 || !rest[0].wordIs(sqlText, "for") {
			return nil, false
		}
		ids := rest[1:]
		// thread_id_list is a comma list of real_ulong_num members —
		// the count must be odd and the last member an id, so a
		// trailing comma fails the claim.
		if len(ids) == 0 || len(ids)%2 == 0 {
			return nil, false
		}
		for i, tok := range ids {
			if i%2 == 0 {
				if tok.kind != tokWord || !isUnsignedDigits(sqlText[tok.start:tok.end]) {
					return nil, false
				}
				continue
			}
			if tok.kind != tokPunct || tok.text(sqlText) != "," {
				return nil, false
			}
		}
		node := &ast.SetResourceGroupStmt{Name: ast.NewCIStr(name)}
		node.SetText(charset.FindEncoding(""), strings.TrimSpace(sqlText))
		return &mysqlResourceGroupStatement{StmtNode: node, setFor: true}, true
	}
	return nil, false
}

// mysqlResourceGroupName validates the group identity token — an
// unquoted word or a backtick-quoted identifier. Unquoted, the word
// must survive upstream's ident rule: every keyword outside the
// ident_keyword family (mysqlResourceGroupReservedWords, generated
// from the 8.4.10 grammar) needs backticks, while ident_keyword members
// — resource, type, vcpu, thread_priority, enable, disable, user and
// hundreds more — are legal unquoted names. A pure-digit token stays
// unclaimed either way.
func mysqlResourceGroupName(sqlText string, tok sqlTok) (string, bool) {
	switch tok.kind {
	case tokQIdent:
		return tok.identText(sqlText), true
	case tokWord:
		raw := sqlText[tok.start:tok.end]
		if _, reserved := mysqlResourceGroupReservedWords[strings.ToLower(raw)]; reserved {
			return "", false
		}
		if isUnsignedDigits(raw) {
			return "", false
		}
		return strings.ToLower(raw), true
	}
	return "", false
}

// mysqlRGCreateOptions consumes the CREATE tail. MySQL's grammar is
// positional: `TYPE opt_equal {SYSTEM|USER}` is a required first
// member (the `=` is optional), then the optional members in fixed
// order — VCPU, THREAD_PRIORITY, ENABLE|DISABLE. A member out of
// position, repeated, or unrecognized fails the whole claim so the
// failure keeps the base grammar's diagnostics — the same verdict
// MySQL's own parser gives.
func mysqlRGCreateOptions(sqlText string, toks []sqlTok) (*spec.MySQLResourceGroupDeclaration, bool) {
	i := 0
	if i >= len(toks) || !toks[i].wordIs(sqlText, "type") {
		return nil, false
	}
	i++
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
		i++
	}
	if i >= len(toks) || toks[i].kind != tokWord {
		return nil, false
	}
	var groupType spec.MySQLResourceGroupType
	switch {
	case toks[i].wordIs(sqlText, "system"):
		groupType = spec.MySQLResourceGroupTypeSystem
	case toks[i].wordIs(sqlText, "user"):
		groupType = spec.MySQLResourceGroupTypeUser
	default:
		return nil, false
	}
	i++
	declaration := &spec.MySQLResourceGroupDeclaration{
		Options: []spec.MySQLResourceGroupOption{
			{Kind: spec.MySQLResourceGroupOptionType, GroupType: groupType},
		},
	}
	next, ok := mysqlRGOptionTail(sqlText, toks, i, declaration, false)
	if !ok || next != len(toks) {
		return nil, false
	}
	return declaration, true
}

// mysqlRGAlterOptions consumes the ALTER tail — the same fixed-order
// optional members as CREATE minus TYPE (unalterable), plus a FORCE
// tail. Every slot is optional, so a bare `ALTER RESOURCE GROUP g`
// claims as a valid no-op; a member out of position or repeated fails
// the claim.
func mysqlRGAlterOptions(sqlText string, toks []sqlTok) (*spec.MySQLResourceGroupDeclaration, bool) {
	declaration := &spec.MySQLResourceGroupDeclaration{}
	next, ok := mysqlRGOptionTail(sqlText, toks, 0, declaration, true)
	if !ok || next != len(toks) {
		return nil, false
	}
	return declaration, true
}

// mysqlRGOptionTail consumes the positional optional members in their
// fixed grammar order — `VCPU [=] spec[,spec]`, `THREAD_PRIORITY [=]
// [-]N`, `ENABLE|DISABLE`, and (alter only) `FORCE` — starting at toks[i].
// Each member occupies a single slot, so repetition or a member out of
// position leaves tokens unconsumed and the caller fails the claim.
func mysqlRGOptionTail(sqlText string, toks []sqlTok, i int, declaration *spec.MySQLResourceGroupDeclaration, allowForce bool) (int, bool) {
	if i < len(toks) && toks[i].wordIs(sqlText, "vcpu") {
		specs, next, ok := mysqlRGVCPUList(sqlText, toks, i+1)
		if !ok {
			return i, false
		}
		i = next
		declaration.Options = append(declaration.Options, spec.MySQLResourceGroupOption{
			Kind: spec.MySQLResourceGroupOptionVCPU, VCPUSpecs: specs,
		})
	}
	if i < len(toks) && toks[i].wordIs(sqlText, "thread_priority") {
		value, next, ok := mysqlRGSignedInt(sqlText, toks, i+1)
		if !ok {
			return i, false
		}
		i = next
		declaration.Options = append(declaration.Options, spec.MySQLResourceGroupOption{
			Kind: spec.MySQLResourceGroupOptionThreadPriority, IntValue: &value,
		})
	}
	if i < len(toks) && (toks[i].wordIs(sqlText, "enable") || toks[i].wordIs(sqlText, "disable")) {
		enabled := toks[i].wordIs(sqlText, "enable")
		i++
		declaration.Options = append(declaration.Options, spec.MySQLResourceGroupOption{
			Kind: spec.MySQLResourceGroupOptionEnabled, Enabled: &enabled,
		})
	}
	if allowForce && i < len(toks) && toks[i].wordIs(sqlText, "force") {
		i++
		declaration.Options = append(declaration.Options, spec.MySQLResourceGroupOption{
			Kind: spec.MySQLResourceGroupOptionForce,
		})
	}
	return i, true
}

// mysqlRGVCPUList consumes `[=] spec` members where spec is `N` or
// `N - N` of unsigned ints — upstream's vcpu_range_list joins members
// through opt_comma, so both `0,1` and the comma-free `0 1` are legal.
// A comma must be followed by a member (`0,` fails); an inverted range
// stays in the declaration — ER_INVALID_VCPU_RANGE is upstream's
// semantic check, not a parse verdict, so the recognizer keeps the
// member verbatim for the rules layer to adjudicate.
func mysqlRGVCPUList(sqlText string, toks []sqlTok, i int) ([]spec.MySQLResourceGroupVCPUSpec, int, bool) {
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
		i++
	}
	var specs []spec.MySQLResourceGroupVCPUSpec
	for {
		if i >= len(toks) || toks[i].kind != tokWord {
			return nil, i, false
		}
		lo, ok := mysqlRGUnsigned(sqlText, toks[i])
		if !ok {
			return nil, i, false
		}
		i++
		member := spec.MySQLResourceGroupVCPUSpec{Lo: lo, Hi: lo}
		if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "-" {
			i++
			if i >= len(toks) || toks[i].kind != tokWord {
				return nil, i, false
			}
			hi, ok := mysqlRGUnsigned(sqlText, toks[i])
			if !ok {
				return nil, i, false
			}
			i++
			member.Hi = hi
		}
		specs = append(specs, member)
		// opt_comma: an optional `,`, then another member — or the list
		// ends when neither follows.
		if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "," {
			i++
			if i >= len(toks) || toks[i].kind != tokWord || !isUnsignedDigits(sqlText[toks[i].start:toks[i].end]) {
				return nil, i, false
			}
			continue
		}
		if i < len(toks) && toks[i].kind == tokWord && isUnsignedDigits(sqlText[toks[i].start:toks[i].end]) {
			continue
		}
		return specs, i, true
	}
}

// mysqlRGSignedInt consumes `[=] [-]N` — THREAD_PRIORITY's signed
// operand. The grammar's signed_num production takes an optional minus
// only, so a `+` operand stays unclaimed.
func mysqlRGSignedInt(sqlText string, toks []sqlTok, i int) (int64, int, bool) {
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "=" {
		i++
	}
	neg := false
	if i < len(toks) && toks[i].kind == tokPunct && toks[i].text(sqlText) == "-" {
		neg = true
		i++
	}
	if i >= len(toks) || toks[i].kind != tokWord {
		return 0, i, false
	}
	raw := sqlText[toks[i].start:toks[i].end]
	if !isUnsignedDigits(raw) {
		return 0, i, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, i, false
	}
	if neg {
		value = -value
	}
	return value, i + 1, true
}

// mysqlRGUnsigned parses one NUM member — the grammar's unsigned
// operand is a u64 literal; anything beyond it stays unclaimed for the
// base diagnostics. The cpu_id_t narrowing happens on the semantic
// side (spec.MySQLResourceGroupEffectiveVCPU), not here.
func mysqlRGUnsigned(sqlText string, tok sqlTok) (uint64, bool) {
	raw := sqlText[tok.start:tok.end]
	if !isUnsignedDigits(raw) {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func isUnsignedDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return false
		}
	}
	return true
}
