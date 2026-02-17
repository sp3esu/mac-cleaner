package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// mockScanner creates a Scanner using NewScanner with the given behavior.
func mockScanner(id, name string, results []scan.CategoryResult, err error) Scanner {
	return NewScanner(ScannerInfo{ID: id, Name: name}, func() ([]scan.CategoryResult, error) {
		return results, err
	})
}

// drainEvents reads all events from the events channel and returns them.
func drainEvents(events <-chan ScanEvent) []ScanEvent {
	var collected []ScanEvent
	for e := range events {
		collected = append(collected, e)
	}
	return collected
}

// --- RegisterDefaults tests (migrated from old DefaultScanners tests) ---

func TestRegisterDefaults_Count(t *testing.T) {
	eng := New()
	RegisterDefaults(eng)
	cats := eng.Categories()
	if len(cats) != 9 {
		t.Errorf("expected 9 default scanners, got %d", len(cats))
	}
}

func TestRegisterDefaults_UniqueIDs(t *testing.T) {
	eng := New()
	RegisterDefaults(eng)
	seen := map[string]bool{}
	for _, info := range eng.Categories() {
		if seen[info.ID] {
			t.Errorf("duplicate scanner ID: %s", info.ID)
		}
		seen[info.ID] = true
	}
}

func TestRegisterDefaults_HaveNames(t *testing.T) {
	eng := New()
	RegisterDefaults(eng)
	for _, info := range eng.Categories() {
		if info.Name == "" {
			t.Errorf("scanner %q has empty Name", info.ID)
		}
	}
}

func TestRegisterDefaults_HaveScanFn(t *testing.T) {
	eng := New()
	RegisterDefaults(eng)
	// Verify by calling Run() — it won't panic if ScanFn is set.
	// We don't check the actual results since they depend on the filesystem.
	for _, info := range eng.Categories() {
		_, err := eng.Run(context.Background(), info.ID)
		// Error is OK (no files found), panic is not.
		_ = err
	}
}

// --- ScanAll tests (migrated to channel-based API) ---

func TestScanAll_AggregatesResults(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1", TotalSize: 100},
	}, nil))
	eng.Register(mockScanner("b", "B", []scan.CategoryResult{
		{Category: "b-1", TotalSize: 200},
		{Category: "b-2", TotalSize: 300},
	}, nil))

	events, done := eng.ScanAll(context.Background(), nil)
	// Drain events to unblock the goroutine.
	drainEvents(events)
	result := <-done

	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}
	if result.Results[0].Category != "a-1" || result.Results[1].Category != "b-1" || result.Results[2].Category != "b-2" {
		t.Errorf("unexpected result order: %v", result.Results)
	}
}

func TestScanAll_SkipsErroredScanners(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("ok", "OK", []scan.CategoryResult{
		{Category: "ok-1", TotalSize: 100},
	}, nil))
	eng.Register(mockScanner("fail", "Fail", nil, errors.New("boom")))
	eng.Register(mockScanner("ok2", "OK2", []scan.CategoryResult{
		{Category: "ok2-1", TotalSize: 50},
	}, nil))

	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	result := <-done

	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results (skipping errored), got %d", len(result.Results))
	}
}

func TestScanAll_AppliesSkipSet(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "keep-me", TotalSize: 100},
		{Category: "skip-me", TotalSize: 200},
	}, nil))

	events, done := eng.ScanAll(context.Background(), map[string]bool{"skip-me": true})
	drainEvents(events)
	result := <-done

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result after skip, got %d", len(result.Results))
	}
	if result.Results[0].Category != "keep-me" {
		t.Errorf("expected keep-me, got %q", result.Results[0].Category)
	}
}

func TestScanAll_ProgressEvents(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1"},
	}, nil))
	eng.Register(mockScanner("b", "B", nil, errors.New("fail")))

	events, done := eng.ScanAll(context.Background(), nil)

	var collected []ScanEvent
	for e := range events {
		collected = append(collected, e)
	}
	<-done // drain done channel

	// Expect: start_a, done_a, start_b, error_b
	if len(collected) != 4 {
		t.Fatalf("expected 4 events, got %d", len(collected))
	}

	expected := []struct {
		typ string
		id  string
	}{
		{EventScannerStart, "a"},
		{EventScannerDone, "a"},
		{EventScannerStart, "b"},
		{EventScannerError, "b"},
	}

	for i, exp := range expected {
		if collected[i].Type != exp.typ {
			t.Errorf("event[%d]: expected type %q, got %q", i, exp.typ, collected[i].Type)
		}
		if collected[i].ScannerID != exp.id {
			t.Errorf("event[%d]: expected scanner %q, got %q", i, exp.id, collected[i].ScannerID)
		}
	}

	// Done event should carry results.
	if len(collected[1].Results) != 1 {
		t.Errorf("done event should have 1 result, got %d", len(collected[1].Results))
	}

	// Error event should carry error.
	if collected[3].Err == nil {
		t.Error("error event should carry non-nil Err")
	}
}

func TestScanAll_EmptyScanners(t *testing.T) {
	eng := New()
	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	result := <-done

	if len(result.Results) != 0 {
		t.Errorf("expected 0 results from empty engine, got %d", len(result.Results))
	}
}

// --- FilterSkipped tests (unchanged, package-level utility) ---

func TestFilterSkipped_EmptySkip(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, nil)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_FiltersMatching(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}, {Category: "c"}}
	got := FilterSkipped(results, map[string]bool{"b": true})
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	for _, r := range got {
		if r.Category == "b" {
			t.Error("category 'b' should have been filtered")
		}
	}
}

func TestFilterSkipped_NonMatchingSkip(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, map[string]bool{"z": true})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_NilResults(t *testing.T) {
	got := FilterSkipped(nil, map[string]bool{"a": true})
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestFilterSkipped_AllSkipped(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, map[string]bool{"a": true, "b": true})
	if len(got) != 0 {
		t.Errorf("expected 0 results after skipping all, got %d", len(got))
	}
}

// --- New tests ---

func TestScanAll_ContextCancellation(t *testing.T) {
	blocker := make(chan struct{})
	eng := New()
	eng.Register(NewScanner(ScannerInfo{ID: "slow", Name: "Slow"}, func() ([]scan.CategoryResult, error) {
		<-blocker // block until test releases
		return []scan.CategoryResult{{Category: "slow-1"}}, nil
	}))

	ctx, cancel := context.WithCancel(context.Background())

	events, done := eng.ScanAll(ctx, nil)

	// Wait for the start event to confirm goroutine is running.
	select {
	case evt, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before start event")
		}
		if evt.Type != EventScannerStart {
			t.Fatalf("expected start event, got %q", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for start event")
	}

	// Cancel the context while scanner is blocked.
	cancel()
	// Release the blocker so the scanner goroutine can return.
	close(blocker)

	// Events channel should close without hanging.
	select {
	case _, ok := <-events:
		if ok {
			// May get one more event; drain remaining.
			for range events {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("events channel did not close after cancellation")
	}

	// Done channel should also close.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("done channel did not close after cancellation")
	}
}

func TestScanAll_ProducesToken(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1"},
	}, nil))

	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	result := <-done

	if result.Token == "" {
		t.Error("expected non-empty token from ScanAll")
	}
}

func TestRun_SingleScanner(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1", TotalSize: 100},
	}, nil))
	eng.Register(mockScanner("b", "B", []scan.CategoryResult{
		{Category: "b-1", TotalSize: 200},
	}, nil))

	results, err := eng.Run(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Category != "a-1" {
		t.Errorf("expected category a-1, got %q", results[0].Category)
	}
}

func TestRun_ScannerNotFound(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", nil, nil))

	_, err := eng.Run(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent scanner")
	}
	if err.Error() != `scanner "nonexistent" not found` {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRun_PropagatesError(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("fail", "Fail", nil, errors.New("disk error")))

	_, err := eng.Run(context.Background(), "fail")
	if err == nil {
		t.Fatal("expected error from failing scanner")
	}

	var scanErr *ScanError
	if !errors.As(err, &scanErr) {
		t.Fatalf("expected *ScanError, got %T", err)
	}
	if scanErr.ScannerID != "fail" {
		t.Errorf("expected scanner ID 'fail', got %q", scanErr.ScannerID)
	}
}

func TestCleanup_ValidToken(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1", Entries: []scan.ScanEntry{
			{Path: "/nonexistent/test/path", Size: 100},
		}},
	}, nil))

	// Scan to get a token.
	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	scanResult := <-done

	// Cleanup with the valid token.
	cleanEvents, cleanDone := eng.Cleanup(context.Background(), scanResult.Token, nil)
	for range cleanEvents {
	}
	cleanResult := <-cleanDone

	if cleanResult.Err != nil {
		t.Fatalf("unexpected error: %v", cleanResult.Err)
	}
	// The path doesn't exist, so cleanup will report failures for the
	// non-existent paths — that's fine for testing the plumbing.
}

func TestCleanup_InvalidToken(t *testing.T) {
	eng := New()

	cleanEvents, cleanDone := eng.Cleanup(context.Background(), "bogus-token", nil)
	for range cleanEvents {
	}
	cleanResult := <-cleanDone

	if cleanResult.Err == nil {
		t.Fatal("expected error for invalid token")
	}

	var tokenErr *TokenError
	if !errors.As(cleanResult.Err, &tokenErr) {
		t.Fatalf("expected *TokenError, got %T: %v", cleanResult.Err, cleanResult.Err)
	}
}

func TestCleanup_TokenConsumed(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1"},
	}, nil))

	// Scan to get a token.
	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	scanResult := <-done

	// First cleanup consumes the token.
	cleanEvents, cleanDone := eng.Cleanup(context.Background(), scanResult.Token, nil)
	for range cleanEvents {
	}
	firstResult := <-cleanDone
	if firstResult.Err != nil {
		t.Fatalf("first cleanup unexpected error: %v", firstResult.Err)
	}

	// Second cleanup with the same token should fail (replay protection).
	cleanEvents2, cleanDone2 := eng.Cleanup(context.Background(), scanResult.Token, nil)
	for range cleanEvents2 {
	}
	secondResult := <-cleanDone2

	if secondResult.Err == nil {
		t.Fatal("expected error on second cleanup (token already consumed)")
	}

	var tokenErr *TokenError
	if !errors.As(secondResult.Err, &tokenErr) {
		t.Fatalf("expected *TokenError, got %T: %v", secondResult.Err, secondResult.Err)
	}
}

func TestCleanup_PartialCategories(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1", Description: "Cat A1", Entries: []scan.ScanEntry{
			{Path: "/nonexistent/a1", Size: 100},
		}},
		{Category: "b-1", Description: "Cat B1", Entries: []scan.ScanEntry{
			{Path: "/nonexistent/b1", Size: 200},
		}},
	}, nil))

	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	scanResult := <-done

	// Cleanup only category "a-1".
	cleanEvents, cleanDone := eng.Cleanup(context.Background(), scanResult.Token, []string{"a-1"})

	var cleanupEvts []CleanupEvent
	for evt := range cleanEvents {
		cleanupEvts = append(cleanupEvts, evt)
	}
	cleanResult := <-cleanDone

	if cleanResult.Err != nil {
		t.Fatalf("unexpected error: %v", cleanResult.Err)
	}

	// Verify that only "a-1" category events were emitted (not "b-1").
	for _, evt := range cleanupEvts {
		if evt.Category == "Cat B1" {
			t.Error("cleanup should not have processed category b-1")
		}
	}
}

func TestCategories_ReturnsRegisteredInfo(t *testing.T) {
	eng := New()
	eng.Register(NewScanner(ScannerInfo{
		ID:          "test-1",
		Name:        "Test One",
		Description: "First test scanner",
		CategoryIDs: []string{"t1-a", "t1-b"},
	}, func() ([]scan.CategoryResult, error) { return nil, nil }))
	eng.Register(NewScanner(ScannerInfo{
		ID:          "test-2",
		Name:        "Test Two",
		Description: "Second test scanner",
		CategoryIDs: []string{"t2-a"},
	}, func() ([]scan.CategoryResult, error) { return nil, nil }))

	cats := eng.Categories()
	if len(cats) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(cats))
	}
	if cats[0].ID != "test-1" || cats[0].Name != "Test One" {
		t.Errorf("unexpected first scanner info: %+v", cats[0])
	}
	if cats[1].ID != "test-2" || cats[1].Name != "Test Two" {
		t.Errorf("unexpected second scanner info: %+v", cats[1])
	}
	if len(cats[0].CategoryIDs) != 2 {
		t.Errorf("expected 2 category IDs for test-1, got %d", len(cats[0].CategoryIDs))
	}
}

func TestEngine_Register(t *testing.T) {
	eng := New()
	if len(eng.Categories()) != 0 {
		t.Error("new engine should have 0 scanners")
	}

	eng.Register(mockScanner("x", "X", nil, nil))
	if len(eng.Categories()) != 1 {
		t.Error("expected 1 scanner after Register")
	}

	eng.Register(mockScanner("y", "Y", nil, nil))
	if len(eng.Categories()) != 2 {
		t.Error("expected 2 scanners after second Register")
	}
}

func TestScanAll_ContextCancelDuringScan(t *testing.T) {
	callCount := 0
	eng := New()
	ctx, cancel := context.WithCancel(context.Background())

	// First scanner succeeds and then cancels the context.
	eng.Register(NewScanner(ScannerInfo{ID: "first", Name: "First"}, func() ([]scan.CategoryResult, error) {
		callCount++
		cancel() // cancel after first scanner completes
		return []scan.CategoryResult{{Category: "first-1"}}, nil
	}))
	eng.Register(NewScanner(ScannerInfo{ID: "second", Name: "Second"}, func() ([]scan.CategoryResult, error) {
		callCount++
		return []scan.CategoryResult{{Category: "second-1"}}, nil
	}))

	events, done := eng.ScanAll(ctx, nil)

	// Drain both channels.
	for range events {
	}
	<-done

	// The second scanner may or may not run depending on timing,
	// but channels must close without hanging.
	// The key assertion is that we reach this point without deadlock.
}

func TestCleanup_ContextCancellation(t *testing.T) {
	eng := New()
	eng.Register(mockScanner("a", "A", []scan.CategoryResult{
		{Category: "a-1", Entries: []scan.ScanEntry{
			{Path: "/nonexistent/path1", Size: 100},
		}},
	}, nil))

	events, done := eng.ScanAll(context.Background(), nil)
	drainEvents(events)
	scanResult := <-done

	// Cancel cleanup immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cleanEvents, cleanDone := eng.Cleanup(ctx, scanResult.Token, nil)

	// Channels should close without hanging.
	select {
	case <-cleanDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup done channel did not close after cancellation")
	}
	for range cleanEvents {
	}
}

func TestStoreResults_SingleTokenPolicy(t *testing.T) {
	eng := New()

	// Store first set of results.
	token1 := eng.storeResults([]scan.CategoryResult{{Category: "first"}})
	if token1 == "" {
		t.Fatal("expected non-empty token1")
	}

	// Store second set — should invalidate the first.
	token2 := eng.storeResults([]scan.CategoryResult{{Category: "second"}})
	if token2 == "" {
		t.Fatal("expected non-empty token2")
	}
	if token1 == token2 {
		t.Error("expected different tokens for different calls")
	}

	// First token should now be invalid.
	_, err := eng.validateToken(token1)
	if err == nil {
		t.Error("expected error for invalidated token1")
	}
	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("expected *TokenError, got %T", err)
	}

	// Second token should still be valid.
	results, err := eng.validateToken(token2)
	if err != nil {
		t.Fatalf("unexpected error for token2: %v", err)
	}
	if len(results) != 1 || results[0].Category != "second" {
		t.Errorf("unexpected results for token2: %v", results)
	}
}

// --- Error type tests ---

func TestScanError_ErrorsAs(t *testing.T) {
	orig := errors.New("disk failure")
	scanErr := &ScanError{ScannerID: "test", Err: orig}

	var target *ScanError
	if !errors.As(scanErr, &target) {
		t.Fatal("errors.As should match *ScanError")
	}
	if target.ScannerID != "test" {
		t.Errorf("expected scanner ID 'test', got %q", target.ScannerID)
	}

	// Test Unwrap.
	if !errors.Is(scanErr, orig) {
		t.Error("errors.Is should match the wrapped error")
	}
}

func TestCancelledError_String(t *testing.T) {
	err := &CancelledError{Operation: "scan"}
	if err.Error() != "scan cancelled" {
		t.Errorf("unexpected error string: %q", err.Error())
	}
}

func TestTokenError_String(t *testing.T) {
	err := &TokenError{Token: "abc123", Reason: "expired"}
	expected := "invalid token abc123: expired"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
