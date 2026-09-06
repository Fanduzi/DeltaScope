// Package connresolve verifies PostgreSQL omitted-port defaulting.
// input: dialect, configured port, and whether the caller set the port explicitly
// output: 5432 for explicit PostgreSQL when the port was omitted
// pos: application tests at the ApplyPostgreSQLDefaultPort seam
// note: if this file changes, update this header and module README.md.
package connresolve

import "testing"

func TestApplyPostgreSQLDefaultPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dialect  string
		port     int
		explicit bool
		want     int
	}{
		{name: "postgresql omitted", dialect: "postgresql", port: 3306, want: 5432},
		{name: "postgresql explicit", dialect: "postgresql", port: 3306, explicit: true, want: 3306},
		{name: "mysql omitted", dialect: "mysql", port: 3306, want: 3306},
		{name: "auto-detect omitted", dialect: "", port: 3306, want: 3306},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ApplyPostgreSQLDefaultPort(tt.dialect, tt.port, tt.explicit)
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
