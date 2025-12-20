package domain

type UserInterest struct {
	UserEmail string
	HighValue *int
	LowValue  *int
}

type Notification struct {
	UserEmail   string `json:"user_email"`
	AirportCode string `json:"airport_code"`
	Message     string `json:"message"`
}
