package runtime

import "fmt"

// Ledger keeps runtime history append-only so later decisions can be audited deterministically.
type Ledger struct {
	runID   RunID
	entries []LedgerEntry
	ids     map[LedgerEntryID]struct{}
}

// NewLedger binds a ledger to one run and prevents cross-run history mixing.
func NewLedger(runID RunID) (*Ledger, error) {
	if err := runID.Validate(); err != nil {
		return nil, err
	}

	return &Ledger{
		runID:   runID,
		entries: []LedgerEntry{},
		ids:     map[LedgerEntryID]struct{}{},
	}, nil
}

// RunID exposes ledger ownership without allowing callers to mutate it.
func (l *Ledger) RunID() RunID {
	if l == nil {
		return ""
	}

	return l.runID
}

// Append validates and appends one immutable runtime fact without rewriting existing history.
func (l *Ledger) Append(entry LedgerEntry) error {
	if l == nil {
		return fmt.Errorf("ledger is nil")
	}

	if err := entry.Validate(); err != nil {
		return err
	}

	if entry.RunID != l.runID {
		return fmt.Errorf("ledger entry run id %q does not match ledger run id %q", string(entry.RunID), string(l.runID))
	}

	if _, exists := l.ids[entry.ID]; exists {
		return fmt.Errorf("ledger entry %q already exists", string(entry.ID))
	}

	l.entries = append(l.entries, entry)
	l.ids[entry.ID] = struct{}{}

	return nil
}

// Entries returns a copy so callers cannot mutate append-only ledger history.
func (l *Ledger) Entries() []LedgerEntry {
	if l == nil {
		return nil
	}

	result := make([]LedgerEntry, len(l.entries))
	copy(result, l.entries)

	return result
}

// Len provides deterministic size checks without exposing internal storage.
func (l *Ledger) Len() int {
	if l == nil {
		return 0
	}

	return len(l.entries)
}

// Last gives orchestration access to the newest fact without requiring slice ownership.
func (l *Ledger) Last() (LedgerEntry, bool) {
	if l == nil || len(l.entries) == 0 {
		return LedgerEntry{}, false
	}

	return l.entries[len(l.entries)-1], true
}

// Find enables audit lookups by stable entry identity without exposing mutable internals.
func (l *Ledger) Find(id LedgerEntryID) (LedgerEntry, bool) {
	if l == nil {
		return LedgerEntry{}, false
	}

	for _, entry := range l.entries {
		if entry.ID == id {
			return entry, true
		}
	}

	return LedgerEntry{}, false
}
