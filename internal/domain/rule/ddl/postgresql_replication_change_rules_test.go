package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPublicationActionList(t *testing.T) {
	for _, item := range []struct {
		value string
		valid bool
	}{
		{"", true}, {"INSERT, insert, truncate", true}, {`"insert", update`, true},
		{`"INSERT"`, false}, {"insert,", false}, {"insert update", false}, {"insert,,update", false},
	} {
		if got := validPGPublicationActions(item.value); got != item.valid {
			t.Errorf("publication list %q accepted = %v, want %v", item.value, got, item.valid)
		}
	}
}

func TestPGSubscriptionPublicationOptions(t *testing.T) {
	prior := &spec.PGSubscriptionChangeState{Publications: []string{"A,B", "other"}}
	change := &spec.PGSubscriptionChange{Action: "add_publication", Publications: []string{"A,B", "A,B"}}
	if reason, fact := assessPGSubscriptionOptions(change, prior); reason != "subscription_publication_duplicate" || fact != "" {
		t.Fatalf("duplicate before disabled refresh = (%q, %q)", reason, fact)
	}
	change.Publications = []string{"a,b"}
	change.Options = []spec.PGReplicationOption{{Name: "refresh", Text: "off", Known: true}}
	if reason, fact := assessPGSubscriptionOptions(change, prior); reason != "" || fact != "" {
		t.Fatalf("case-sensitive publication with refresh disabled = (%q, %q)", reason, fact)
	}
	change.Action, change.Publications = "drop_publication", []string{"other", "A,B"}
	if reason, fact := assessPGSubscriptionOptions(change, prior); reason != "subscription_publications_empty" || fact != "" {
		t.Fatalf("drop all publications = (%q, %q)", reason, fact)
	}
}
