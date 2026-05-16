//go:build postgresql

package postgresql

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractDropTransform(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTransform {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTransform, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "jsonb@plpython3u" {
		t.Errorf("expected object_name jsonb@plpython3u, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "transform" {
		t.Errorf("expected object_type transform, got %q", s.DDL.ObjectType)
	}
	for k := range s.DDL.Options {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "function") || strings.Contains(kl, "fromsql") || strings.Contains(kl, "tosql") {
			t.Errorf("leaked payload key %q in options", k)
		}
	}
}

func TestExtractDropTransformIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TRANSFORM IF EXISTS FOR jsonb LANGUAGE plpython3u CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTransform {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTransform, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

func TestExtractDropAccessMethod(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP ACCESS METHOD heap2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropAccessMethod {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropAccessMethod, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "heap2" {
		t.Errorf("expected object_name heap2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "access_method" {
		t.Errorf("expected object_type access_method, got %q", s.DDL.ObjectType)
	}
}

func TestExtractDropAccessMethodIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP ACCESS METHOD IF EXISTS heap2 CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropAccessMethod {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropAccessMethod, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

func TestExtractAlterLargeObjectOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER LARGE OBJECT 12345 OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterLargeObject {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterLargeObject, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "12345" {
		t.Errorf("expected object_name 12345, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "large_object" {
		t.Errorf("expected object_type large_object, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractCreateTransformDeferred(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))")
	if s.DDL != nil && s.DDL.Operation != "" {
		t.Errorf("CREATE TRANSFORM should be deferred/unsupported, got operation %q", s.DDL.Operation)
	}
}

func TestExtractCreateAccessMethodDeferred(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler")
	if s.DDL != nil && s.DDL.Operation != "" {
		t.Errorf("CREATE ACCESS METHOD should be deferred/unsupported, got operation %q", s.DDL.Operation)
	}
}

func assertNoLeakedBoundaryPayload(t *testing.T, options map[string]string) {
	t.Helper()
	forbidden := []string{
		"jsonb_to_plpython", "plpython_to_jsonb", "heap_tableam_handler",
		"function", "handler", "definition", "body", "query",
		"options", "password", "secret", "token", "fromsql", "tosql",
	}
	for _, key := range forbidden {
		if _, ok := options[key]; ok {
			t.Errorf("leaked payload key %q found in options", key)
		}
	}
	for k, v := range options {
		kl := strings.ToLower(k + "=" + v)
		for _, fb := range forbidden {
			if strings.Contains(kl, strings.ToLower(fb)) {
				t.Errorf("leaked payload value %q in options[%s]=%s", fb, k, v)
			}
		}
	}
}
