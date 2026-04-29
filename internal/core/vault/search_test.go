package vault

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSearchRanksNameBeforeURI(t *testing.T) {
	items := []Item{
		{
			ID:           "1",
			Name:         "Example Login",
			Type:         ItemTypeLogin,
			RevisionDate: time.Now(),
			Login: &Login{
				Username: "user",
				URIs: []URI{
					{URI: "https://example.com"},
				},
			},
		},
		{
			ID:           "2",
			Name:         "Other Site",
			Type:         ItemTypeLogin,
			RevisionDate: time.Now(),
			Login: &Login{
				Username: "other",
				URIs: []URI{
					{URI: "https://example.com/login"},
				},
			},
		},
	}

	idx := BuildIndex(items)
	results := idx.Search("example", 10)

	require.Len(t, results, 2, "should find both items")
	require.Equal(t, "1", results[0].Item.ID, "item with name match should rank first")
	require.Greater(t, results[0].Score, results[1].Score, "name match score should exceed URI match score")
}

func TestSearchDoesNotIndexPasswordsOrTOTP(t *testing.T) {
	items := []Item{
		{
			ID:           "1",
			Name:         "My Secret",
			Type:         ItemTypeLogin,
			RevisionDate: time.Now(),
			Login: &Login{
				Username: "user@example.com",
				Password: "supersecretpassword123",
				TOTP:     "JBSWY3DPEHPK3PXP",
			},
		},
		{
			ID:           "2",
			Name:         "Another Login",
			Type:         ItemTypeLogin,
			RevisionDate: time.Now(),
			Login: &Login{
				Username: "admin",
				Password: "adminpass",
				TOTP:     "GEZDGNBVGY3TQOJQ",
			},
		},
	}

	idx := BuildIndex(items)

	// Searching for the password value should return no results.
	results := idx.Search("supersecretpassword123", 10)
	require.Empty(t, results, "password must not be indexed")

	// Searching for the TOTP value should return no results.
	results = idx.Search("JBSWY3DPEHPK3PXP", 10)
	require.Empty(t, results, "TOTP must not be indexed")

	// Sanity check: searching for username still works.
	results = idx.Search("user@example.com", 10)
	require.Len(t, results, 1, "username should still be searchable")
}

func TestSearchDoesNotIndexCardNumberCodeOrHiddenFields(t *testing.T) {
	items := []Item{
		{
			ID:           "1",
			Name:         "My Card",
			Type:         ItemTypeCard,
			RevisionDate: time.Now(),
			Card: &Card{
				CardholderName: "Alice",
				Brand:          "Visa",
				Number:         "4111111111111111",
				Code:           "123",
			},
			Fields: []Field{
				{Name: "note", Value: "visible value", Hidden: false},
				{Name: "pin", Value: "hidden-pin-999", Hidden: true},
			},
		},
	}

	idx := BuildIndex(items)

	// Card.Number must not be indexed.
	results := idx.Search("4111111111111111", 10)
	require.Empty(t, results, "card number must not be indexed")

	// Card.Code must not be indexed.
	results = idx.Search("123", 10)
	require.Empty(t, results, "card code must not be indexed")

	// Hidden field value must not be indexed.
	results = idx.Search("hidden-pin-999", 10)
	require.Empty(t, results, "hidden field value must not be indexed")

	// Visible field value should still be searchable.
	results = idx.Search("visible value", 10)
	require.Len(t, results, 1, "visible field value should be searchable")

	// CardholderName should be searchable.
	results = idx.Search("Alice", 10)
	require.Len(t, results, 1, "cardholder name should be searchable")

}

func TestSearchDoesNotIndexIdentityGovernmentIDs(t *testing.T) {
	items := []Item{
		{
			ID:           "1",
			Name:         "Identity",
			Type:         ItemTypeIdentity,
			RevisionDate: time.Now(),
			Identity: &Identity{
				FirstName:      "Alice",
				SSN:            "999-99-9999",
				PassportNumber: "P123456789",
				LicenseNumber:  "D1234567",
			},
		},
	}

	idx := BuildIndex(items)
	require.Empty(t, idx.Search("999-99-9999", 10), "SSN must not be indexed")
	require.Empty(t, idx.Search("P123456789", 10), "passport number must not be indexed")
	require.Empty(t, idx.Search("D1234567", 10), "license number must not be indexed")
	require.Len(t, idx.Search("Alice", 10), 1, "non-secret identity name should be searchable")
}
