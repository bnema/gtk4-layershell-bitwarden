package vault

import "time"

// ItemType represents the type of a vault item.
type ItemType string

const (
	ItemTypeLogin      ItemType = "login"
	ItemTypeSecureNote ItemType = "secure_note"
	ItemTypeCard       ItemType = "card"
	ItemTypeIdentity   ItemType = "identity"
)

// SyncStatus represents the sync state of a vault item.
type SyncStatus string

const (
	SyncStatusSynced   SyncStatus = "synced"
	SyncStatusPending  SyncStatus = "pending"
	SyncStatusConflict SyncStatus = "conflict"
)

// Item represents a Bitwarden vault item.
type Item struct {
	ID           string       `json:"id"`
	FolderID     string       `json:"folderId"`
	Type         ItemType     `json:"type"`
	Name         string       `json:"name"`
	Notes        string       `json:"notes"`
	Favorite     bool         `json:"favorite"`
	Deleted      bool         `json:"deleted"`
	RevisionDate time.Time    `json:"revisionDate"`
	SyncStatus   SyncStatus   `json:"syncStatus"`
	ConflictID   string       `json:"conflictId,omitempty"`
	Login        *Login       `json:"login,omitempty"`
	SecureNote   *SecureNote  `json:"secureNote,omitempty"`
	Card         *Card        `json:"card,omitempty"`
	Identity     *Identity    `json:"identity,omitempty"`
	Fields       []Field      `json:"fields,omitempty"`
	Attachments  []Attachment `json:"attachments,omitempty"`
}

// Login represents a login item's secrets.
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     string `json:"totp"`
	URIs     []URI  `json:"uris,omitempty"`
}

// SecureNote represents a secure note item's content.
type SecureNote struct {
	Text string `json:"text"`
}

// Card represents a credit card item's details.
type Card struct {
	CardholderName string `json:"cardholderName"`
	Brand          string `json:"brand"`
	Number         string `json:"number"`
	ExpMonth       string `json:"expMonth"`
	ExpYear        string `json:"expYear"`
	Code           string `json:"code"`
}

// Identity represents an identity item's details.
type Identity struct {
	Title          string `json:"title"`
	FirstName      string `json:"firstName"`
	MiddleName     string `json:"middleName"`
	LastName       string `json:"lastName"`
	SubName        string `json:"subName"`
	Address1       string `json:"address1"`
	Address2       string `json:"address2"`
	Address3       string `json:"address3"`
	City           string `json:"city"`
	State          string `json:"state"`
	PostalCode     string `json:"postalCode"`
	Country        string `json:"country"`
	Company        string `json:"company"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	SSN            string `json:"ssn"`
	PassportNumber string `json:"passportNumber"`
	LicenseNumber  string `json:"licenseNumber"`
	Username       string `json:"username"`
}

// URI represents a URI associated with a login item.
type URI struct {
	URI string `json:"uri"`
}

// Field represents a custom field on a vault item.
type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Hidden bool   `json:"hidden"`
}

// Attachment represents a file attached to a vault item.
type Attachment struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	Size     string `json:"size"`
	Url      string `json:"url"`
}

// Folder represents a folder for organising vault items.
type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
