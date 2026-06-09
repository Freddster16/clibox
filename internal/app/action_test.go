package app

import (
	"strings"
	"testing"
)

func TestBuildSearchQueryTokenizesPlainText(t *testing.T) {
	query := buildSearchQuery("Alice deploy notes")
	for _, want := range []string{
		"(from alice or to alice or subject alice or body alice)",
		"and",
		"(from deploy or to deploy or subject deploy or body deploy)",
		"(from notes or to notes or subject notes or body notes)",
		"order by date desc",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected query to contain %q, got %q", want, query)
		}
	}
	if strings.Contains(query, "Alice") {
		t.Fatalf("expected search query to be normalized to lowercase, got %q", query)
	}
}

func TestBuildSearchQueryDropsBackendKeywords(t *testing.T) {
	query := buildSearchQuery("from alice or before 2026")
	if strings.Contains(query, "from from") || strings.Contains(query, "subject or") || strings.Contains(query, "body before") {
		t.Fatalf("expected backend keywords to be dropped, got %q", query)
	}
	if !strings.Contains(query, "alice") || !strings.Contains(query, "2026") {
		t.Fatalf("expected useful terms to remain, got %q", query)
	}
}
