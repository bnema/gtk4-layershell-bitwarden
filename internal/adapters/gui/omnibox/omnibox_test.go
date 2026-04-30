package omnibox

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/session"
)

func TestNewState_Defaults(t *testing.T) {
	s := NewState()
	require.Equal(t, ModeUnlock, s.Mode)
	require.Equal(t, 0, s.Selected)
	require.Empty(t, s.Rows)
	require.Empty(t, s.Query)
}

func TestMove_ClampsNotWraps(t *testing.T) {
	s := NewState()
	s.Rows = []Row{
		{ID: "1", Title: "One"},
		{ID: "2", Title: "Two"},
		{ID: "3", Title: "Three"},
	}

	// Move down
	s.Move(1)
	require.Equal(t, 1, s.Selected)

	s.Move(1)
	require.Equal(t, 2, s.Selected)

	// Clamp at bottom
	s.Move(1)
	require.Equal(t, 2, s.Selected, "should clamp at last index, not wrap")

	// Move up
	s.Move(-1)
	require.Equal(t, 1, s.Selected)

	s.Move(-1)
	require.Equal(t, 0, s.Selected)

	// Clamp at top
	s.Move(-1)
	require.Equal(t, 0, s.Selected, "should clamp at 0, not wrap")
}

func TestMove_EmptyRows(t *testing.T) {
	s := NewState()
	s.Move(1)
	require.Equal(t, 0, s.Selected)
	s.Move(-1)
	require.Equal(t, 0, s.Selected)
}

func TestSetRows_ResetsSelectionOutOfBounds(t *testing.T) {
	s := NewState()
	s.Selected = 10
	s.SetRows([]Row{
		{ID: "a"},
		{ID: "b"},
	})
	require.Equal(t, 1, s.Selected, "should clamp to last index")

	s.SetRows(nil)
	require.Equal(t, 0, s.Selected)

	s.SetRows([]Row{{ID: "x"}})
	require.Equal(t, 0, s.Selected)
}

func TestSelectedRow(t *testing.T) {
	s := NewState()
	_, ok := s.SelectedRow()
	require.False(t, ok, "no rows should return false")

	s.Rows = []Row{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}
	row, ok := s.SelectedRow()
	require.True(t, ok)
	require.Equal(t, "a", row.ID)

	s.Move(1)
	row, ok = s.SelectedRow()
	require.True(t, ok)
	require.Equal(t, "b", row.ID)
}

func TestOpenDetail(t *testing.T) {
	s := NewState()
	s.Rows = []Row{{ID: "d1", Title: "Detail Item"}}

	s.OpenDetail()
	require.Equal(t, ModeDetail, s.Mode)
	require.Equal(t, "d1", s.DetailID)
	require.Equal(t, 0, s.Selected)
}

func TestOpenDetail_NoRows(t *testing.T) {
	s := NewState()
	s.Mode = ModeSearch
	s.OpenDetail()
	require.Equal(t, ModeSearch, s.Mode, "should not change mode with no rows")
}

func TestBack(t *testing.T) {
	// Back from detail -> search
	s := NewState()
	s.Mode = ModeDetail
	s.DetailID = "some-id"
	s.Back()
	require.Equal(t, ModeSearch, s.Mode)
	require.Empty(t, s.DetailID)

	// Back from form -> search
	s2 := NewState()
	s2.Mode = ModeForm
	s2.Back()
	require.Equal(t, ModeSearch, s2.Mode)

	// Back from search -> stays search
	s3 := NewState()
	s3.Mode = ModeSearch
	s3.Back()
	require.Equal(t, ModeSearch, s3.Mode)
}

func TestSetStatus(t *testing.T) {
	s := NewState()
	st := Status{Text: "Online", Syncing: false}
	s.SetStatus(st)
	require.Equal(t, "Online", s.Status.Text)
}

func TestModeForAuthStatus_KeyringUnavailable(t *testing.T) {
	mode := ModeForAuthStatus(session.KeyringUnavailable, true)
	require.Equal(t, ModeKeyringError, mode)

	mode = ModeForAuthStatus(session.KeyringUnavailable, false)
	require.Equal(t, ModeKeyringError, mode)
}

func TestModeForAuthStatus_LoggedInUnlockAvailable(t *testing.T) {
	mode := ModeForAuthStatus(session.LoggedInUnlockAvailable, true)
	require.Equal(t, ModePINUnlock, mode)

	mode = ModeForAuthStatus(session.LoggedInUnlockAvailable, false)
	require.Equal(t, ModeUnlock, mode, "no email should fall back to ModeUnlock")
}

func TestModeForAuthStatus_LoggedInLocked(t *testing.T) {
	mode := ModeForAuthStatus(session.LoggedInLocked, true)
	require.Equal(t, ModeUnlock, mode)

	mode = ModeForAuthStatus(session.LoggedInLocked, false)
	require.Equal(t, ModeUnlock, mode)
}

func TestModeForAuthStatus_Unauthenticated(t *testing.T) {
	mode := ModeForAuthStatus(session.Unauthenticated, true)
	require.Equal(t, ModeUnlock, mode)

	mode = ModeForAuthStatus(session.Unauthenticated, false)
	require.Equal(t, ModeUnlock, mode)
}

func TestBack_UnlockModesNoOp(t *testing.T) {
	for _, mode := range []Mode{ModeUnlock, ModePINUnlock, ModeKeyringError} {
		s := NewState()
		s.Mode = mode
		s.Back()
		require.Equal(t, mode, s.Mode, "Back should not change mode from unlock/keyring modes")
	}
}
