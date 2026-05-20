package runs

import (
	"testing"
)

func TestSplitSQLStatements(t *testing.T) {
	statements := splitSQLStatements(runSchema)
	if len(statements) != 2 {
		t.Fatalf("splitSQLStatements() returned %d statements, want 2", len(statements))
	}

	if statements[0] == "" {
		t.Fatal("first statement is empty")
	}

	if statements[1] == "" {
		t.Fatal("second statement is empty")
	}
}
