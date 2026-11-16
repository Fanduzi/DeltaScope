package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestCloneColumnOwnsPGGeneration(t *testing.T) {
	value := int64(2147483648)
	in := spec.Column{PGGeneration: &spec.PGColumnGeneration{Mode: "a", Options: []spec.SequenceOption{{Kind: spec.SequenceOptionStartWith, Value: &value}}}}
	out := cloneColumn(in)
	out.PGGeneration.Mode = "d"
	*out.PGGeneration.Options[0].Value = 1
	if in.PGGeneration.Mode != "a" || value != 2147483648 {
		t.Fatal("clone mutated authored generation facts")
	}
}
