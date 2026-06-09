package app

import (
	"fmt"
	"strings"
)

type messageActionKind int

const (
	archiveAction messageActionKind = iota
	deleteAction
)

type messageActionState struct {
	Kind    messageActionKind
	Message message
	Index   int
	Serial  int
	Running bool
}

func (k messageActionKind) verb() string {
	if k == deleteAction {
		return "delete"
	}
	return "archive"
}

func (k messageActionKind) presentParticiple() string {
	if k == deleteAction {
		return "deleting"
	}
	return "archiving"
}

func (k messageActionKind) pastTense() string {
	if k == deleteAction {
		return "Moved to Trash"
	}
	return "Archived"
}

func selectedMessageLabel(msg message) string {
	subject := firstNonEmpty(msg.Subject, "(no subject)")
	if strings.TrimSpace(msg.From) == "" {
		return subject
	}
	return fmt.Sprintf("%s - %s", msg.From, subject)
}

func buildSearchQuery(input string) string {
	terms := searchTerms(input)
	if len(terms) == 0 {
		return ""
	}

	groups := make([]string, 0, len(terms))
	for _, term := range terms {
		groups = append(groups, fmt.Sprintf("(from %s or to %s or subject %s or body %s)", term, term, term, term))
	}
	return strings.Join(groups, " and ") + " order by date desc"
}

func searchTerms(input string) []string {
	fields := strings.Fields(strings.ToLower(input))
	terms := make([]string, 0, min(len(fields), 8))
	seen := map[string]bool{}
	for _, field := range fields {
		term := sanitizeSearchTerm(field)
		if term == "" || seen[term] || isSearchKeyword(term) {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
		if len(terms) == 8 {
			break
		}
	}
	return terms
}

func sanitizeSearchTerm(input string) string {
	input = strings.Trim(input, `"'`+"`()[]{}<>.,;:!?")
	var out strings.Builder
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '@' || r == '.' || r == '_' || r == '+' || r == '-':
			out.WriteRune(r)
		}
	}
	return strings.Trim(out.String(), ".+-_@")
}

func isSearchKeyword(term string) bool {
	switch term {
	case "and", "or", "not", "order", "by", "date", "from", "to", "subject", "body", "flag", "before", "after":
		return true
	default:
		return false
	}
}
