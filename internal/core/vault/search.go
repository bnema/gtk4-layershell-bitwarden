package vault

import (
	"sort"
	"strings"
	"unicode"
)

// ScoredItem pairs a vault Item with its relevance score.
type ScoredItem struct {
	Item  Item
	Score int
}

// keywordEntry holds a pre-indexed keyword for a specific item.
type keywordEntry struct {
	itemIndex int
	keyword   string
	boost     int
}

// SearchIndex provides fast fuzzy search over vault items.
type SearchIndex struct {
	items    []Item
	keywords []keywordEntry
}

// BuildIndex creates a SearchIndex from the given items. All indexable
// text fields are extracted and stored for fast prefix-based search.
func BuildIndex(items []Item) *SearchIndex {
	idx := &SearchIndex{
		items:    items,
		keywords: make([]keywordEntry, 0, len(items)*8),
	}
	for i, item := range items {
		idx.indexItem(i, item)
	}
	return idx
}

// Search returns up to limit items whose indexed keywords match the
// query. Results are sorted by relevance (highest score first).
func (idx *SearchIndex) Search(query string, limit int) []ScoredItem {
	if query == "" || limit <= 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	scores := make(map[int]int) // itemIndex -> bestScore

	for _, entry := range idx.keywords {
		kw := strings.ToLower(entry.keyword)
		score := scoreKeyword(kw, q, entry.boost)
		if score > 0 {
			existing := scores[entry.itemIndex]
			if score > existing {
				scores[entry.itemIndex] = score
			}
		}
	}

	// Convert to a sorted slice. Tie-break on name and ID for deterministic UI.
	results := make([]ScoredItem, 0, len(scores))
	for itemIdx := range scores {
		results = append(results, ScoredItem{Item: idx.items[itemIdx], Score: scores[itemIdx]})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Item.Name != results[j].Item.Name {
			return results[i].Item.Name < results[j].Item.Name
		}
		return results[i].Item.ID < results[j].Item.ID
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// indexItem extracts all indexable keywords from a single item.
func (idx *SearchIndex) indexItem(i int, item Item) {
	// Name — boost 100
	if item.Name != "" {
		idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Name, boost: 100})
	}
	// Notes — boost 10
	if item.Notes != "" {
		idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Notes, boost: 10})
	}
	// Login
	if item.Login != nil {
		if item.Login.Username != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Login.Username, boost: 60})
		}
		for _, u := range item.Login.URIs {
			if u.URI != "" {
				idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: u.URI, boost: 40})
			}
		}
		// Password and TOTP are intentionally NOT indexed.
	}
	// SecureNote
	if item.SecureNote != nil && item.SecureNote.Text != "" {
		idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.SecureNote.Text, boost: 5})
	}
	// Card — index identifying metadata only. Number and Code are intentionally
	// NOT indexed because they are sensitive payment fields.
	if item.Card != nil {
		if item.Card.CardholderName != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Card.CardholderName, boost: 5})
		}
		if item.Card.Brand != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Card.Brand, boost: 5})
		}
		if item.Card.ExpMonth != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Card.ExpMonth, boost: 5})
		}
		if item.Card.ExpYear != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: item.Card.ExpYear, boost: 5})
		}
	}
	// Identity — index non-secret identifying fields only. Government IDs are
	// intentionally NOT indexed.
	if item.Identity != nil {
		identityKeywords := []string{
			item.Identity.Title, item.Identity.FirstName, item.Identity.MiddleName,
			item.Identity.LastName, item.Identity.SubName, item.Identity.Address1,
			item.Identity.Address2, item.Identity.Address3, item.Identity.City,
			item.Identity.State, item.Identity.PostalCode, item.Identity.Country,
			item.Identity.Company, item.Identity.Email, item.Identity.Phone,
			item.Identity.Username,
		}
		for _, kw := range identityKeywords {
			if kw != "" {
				idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: kw, boost: 5})
			}
		}
	}
	// Fields — index only non-hidden
	for _, f := range item.Fields {
		if !f.Hidden && f.Value != "" {
			idx.keywords = append(idx.keywords, keywordEntry{itemIndex: i, keyword: f.Value, boost: 5})
		}
	}
}

// scoreKeyword calculates a score for a single keyword match.
// Returns 0 if no match, otherwise a positive score.
func scoreKeyword(keyword, query string, boost int) int {
	if strings.HasPrefix(keyword, query) {
		// Exact prefix match — highest tier.
		return boost * 3
	}
	if wordPrefixMatch(keyword, query) {
		// Word prefix match — middle tier.
		return boost * 2
	}
	if strings.Contains(keyword, query) {
		// Substring match — lowest tier.
		return boost
	}
	return 0
}

// wordPrefixMatch returns true if query is a prefix of any word in s.
// Words are delimited by whitespace, hyphens, underscores, dots, slashes.
// This allows matching e.g. "ex" against "https://example.com/login" by
// matching the start of each path segment delimited by '/', '_', '-', etc.
func wordPrefixMatch(s, query string) bool {
	// Walk runes and check each word boundary.
	start := 0
	for i, r := range s {
		if isWordBoundary(r) {
			word := s[start:i]
			if strings.HasPrefix(word, query) {
				return true
			}
			start = i + 1
		}
	}
	// Check last word.
	word := s[start:]
	return strings.HasPrefix(word, query)
}

func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '/'
}
