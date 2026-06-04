package postgres

import "testing"

func TestPublicTagHavingClauseRequiresPublishedContent(t *testing.T) {
	if publicTagHavingClause != "HAVING count(ci.id) > 0" {
		t.Fatalf("public tag visibility clause = %q", publicTagHavingClause)
	}
}
