package domain

import "testing"

func TestNewInterestNormalizesAndValidatesThresholds(t *testing.T) {
	low, high := 5, 20
	interest, err := NewInterest(" MARIO@EXAMPLE.COM ", " licc ", &low, &high)
	if err != nil {
		t.Fatalf("NewInterest() error = %v", err)
	}
	if interest.UserEmail != "mario@example.com" || interest.AirportCode != "LICC" {
		t.Fatalf("NewInterest() normalization = %+v", interest)
	}

	if _, err := NewInterest("mario@example.com", "LICC", &high, &low); err == nil {
		t.Fatal("NewInterest() accepted an invalid threshold range")
	}
	if _, err := NewInterest("mario@example.com", "CTA", nil, nil); err == nil {
		t.Fatal("NewInterest() accepted a non-ICAO airport code")
	}
}
