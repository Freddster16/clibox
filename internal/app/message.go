package app

type message struct {
	ID         string
	From       string
	Email      string
	Subject    string
	Date       string
	Preview    string
	Body       string
	BodyLoaded bool
	BodyError  string
	Unread     bool
}
