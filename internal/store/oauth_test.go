package store_test

import (
	"errors"
	"testing"

	"github.com/gesellix/go-trmnl/internal/store"
)

func TestOAuthClientCRUD(t *testing.T) {
	st := openTest(t)

	a, err := st.CreateOAuthClient("google", "Family GCP", "cid-1", "secret-1")
	if err != nil {
		t.Fatal(err)
	}
	if a.Provider != "google" || a.Name != "Family GCP" || a.ClientID != "cid-1" || a.ClientSecret != "secret-1" {
		t.Fatalf("create returned %+v", a)
	}

	got, err := st.GetOAuthClient(a.ID)
	if err != nil || got.ClientID != "cid-1" {
		t.Fatalf("GetOAuthClient: %+v err=%v", got, err)
	}

	// A second client coexists.
	if _, err := st.CreateOAuthClient("google", "Work GCP", "cid-2", "secret-2"); err != nil {
		t.Fatal(err)
	}
	clients, _ := st.ListOAuthClients()
	if len(clients) != 2 {
		t.Fatalf("ListOAuthClients = %d, want 2", len(clients))
	}

	if err := st.DeleteOAuthClient(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOAuthClient(a.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("client still present after delete: %v", err)
	}
}
