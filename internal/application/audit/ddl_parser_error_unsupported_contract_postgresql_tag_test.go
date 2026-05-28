//go:build postgresql

package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestDDLParserErrorUnsupportedContractPostgreSQL(t *testing.T) {
	t.Parallel()

	cases := []parserErrorUnsupportedContractCase{
		{
			Dialect:   spec.DialectPostgreSQL,
			Name:      "postgres_drop_subscription_with_options_not_audited",
			SQL:       "DROP SUBSCRIPTION sub WITH (drop_slot = true)",
			Forbidden: []string{"drop_slot"},
		},
		{
			Dialect:   spec.DialectPostgreSQL,
			Name:      "postgres_pg18_constraint_not_audited",
			SQL:       "ALTER TABLE users ADD CONSTRAINT users_email_nn NOT NULL email NOT VALID",
			Forbidden: []string{"users_email_nn", "NOT VALID"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			assertParserErrorUnsupportedContract(t, tc)
		})
	}
}
