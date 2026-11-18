package scantest

import "testing"

func TestOnDisk(t *testing.T) {
	if got := OnDisk(t); got != 0 {
		t.Errorf("OnDisk() = %d, want 0", got)
	}
	if got := OnDisk(t, 0); got != 0 {
		t.Errorf("OnDisk(0) = %d, want 0", got)
	}
	one := OnDisk(t, 1)
	if one < 512 {
		t.Errorf("OnDisk(1) = %d, want at least one 512-byte block", one)
	}
	if got := OnDisk(t, 1, 1); got != 2*one {
		t.Errorf("OnDisk(1, 1) = %d, want %d", got, 2*one)
	}
	if got := OnDisk(t, 1<<20); got < 1<<20 {
		t.Errorf("OnDisk(1 MiB) = %d, want at least %d", got, 1<<20)
	}
}
