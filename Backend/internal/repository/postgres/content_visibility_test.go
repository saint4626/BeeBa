package postgres

import (
	"testing"

	"beeba.org/internal/domain/content"
)

func TestVisibilityOnlyPublishedUpdateDoesNotRequireModeration(t *testing.T) {
	visibility := "private"

	if ownerUpdateNeedsModeration("published", content.OwnerUpdateInput{Visibility: &visibility}) {
		t.Fatal("published content should not return to moderation when only visibility changes")
	}
}

func TestPublishedMetadataUpdateStillRequiresModeration(t *testing.T) {
	title := "Updated title"

	if !ownerUpdateNeedsModeration("published", content.OwnerUpdateInput{Title: &title}) {
		t.Fatal("published content should return to moderation when metadata changes")
	}
}
