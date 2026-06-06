package device_test

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/store"
)

func TestNormalizeMAC(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF", true},
		{"AA-BB-CC-DD-EE-FF", "AA:BB:CC:DD:EE:FF", true},
		{"  aa:bb:cc:dd:ee:ff  ", "AA:BB:CC:DD:EE:FF", true},
		{"not-a-mac", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, err := device.NormalizeMAC(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("NormalizeMAC(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("NormalizeMAC(%q) expected error", c.in)
		}
	}
}

func TestProvisionCreatesAndIsIdempotent(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	const mac = "AA:BB:CC:DD:EE:FF"
	d, created, err := device.Provision(st, mac, "model-x", "1.5.2")
	if err != nil || !created {
		t.Fatalf("first provision: created=%v err=%v", created, err)
	}
	if d.APIKey == "" {
		t.Error("api_key empty")
	}
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{6}$`).MatchString(d.FriendlyID) {
		t.Errorf("friendly_id %q is not 6-char Crockford base32", d.FriendlyID)
	}

	// Second provision returns the same device without re-creating.
	d2, created2, err := device.Provision(st, mac, "", "")
	if err != nil || created2 {
		t.Fatalf("second provision: created=%v err=%v", created2, err)
	}
	if d2.APIKey != d.APIKey || d2.FriendlyID != d.FriendlyID {
		t.Errorf("credentials changed on re-provision")
	}
}

func TestProvisionGeneratesDistinctKeys(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { _ = st.Close() })

	a, _, _ := device.Provision(st, "AA:BB:CC:DD:EE:01", "", "")
	b, _, _ := device.Provision(st, "AA:BB:CC:DD:EE:02", "", "")
	if a.APIKey == b.APIKey {
		t.Errorf("two devices share an api_key: %q", a.APIKey)
	}
	if a.FriendlyID == b.FriendlyID {
		t.Errorf("two devices share a friendly_id: %q", a.FriendlyID)
	}
}
