package domain

import "testing"

func TestNewUserNormalizesEmailAndRejectsMissingFields(t *testing.T) {
	user, err := NewUser(" Mario ", " Rossi ", " MARIO@EXAMPLE.COM ", "hash", "1234", "12/30", "123")
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}

	if user.Email != "mario@example.com" || user.FirstName != "Mario" || user.LastName != "Rossi" {
		t.Fatalf("NewUser() normalization = %+v", user)
	}

	if _, err := NewUser("", "Rossi", "mario@example.com", "hash", "", "", ""); err == nil {
		t.Fatal("NewUser() accepted an empty first name")
	}
}
