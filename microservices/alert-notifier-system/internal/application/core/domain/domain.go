package domain

type Notification struct {
	UserEmail   string `json:"user_email"`
	AirportCode string `json:"airport_code"`
	Message     string `json:"message"`
}
