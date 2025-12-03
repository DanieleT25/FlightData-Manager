package domain

import (
	"errors"
	"strings"
	"time"
)

type User struct {
	FirstName      string      `json:"first_name"`
	LastName       string      `json:"last_name"`
	BankingDetails CardDetails `json:"card_details"`
	Email          string      `json:"email"`
	PasswordHash   string      `json:"password_hash"`
	RegisteredAt   time.Time   `json:"registered_at"`
}

type CardDetails struct {
	CardNumber     string `json:"card_number"`
	ExpirationDate string `json:"expiration_date"`
	CVV            string `json:"cvv"`
}

func NewUser(firstName, lastName, email, passwordHash, cardNumber, expirationDate, cvv string) (*User, error) {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	email = strings.ToLower(strings.TrimSpace(email))

	if firstName == "" {
		return nil, errors.New("firstName cannot be empty")
	}
	if lastName == "" {
		return nil, errors.New("lastName cannot be empty")
	}
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}
	if passwordHash == "" {
		return nil, errors.New("password cannot be empty")
	}

	return &User{
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		PasswordHash: passwordHash,
		RegisteredAt: time.Now(),
		BankingDetails: CardDetails{
			CardNumber:     cardNumber,
			ExpirationDate: expirationDate,
			CVV:            cvv,
		},
	}, nil
}
