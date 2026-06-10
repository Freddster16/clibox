package app

type message struct {
	ID         string
	From       string
	Email      string
	Subject    string
	Date       string
	Preview    string
	Body       string
	Images     []messageImage
	BodyLoaded bool
	BodyError  string
	Unread     bool
}

type messageImage struct {
	Name        string
	ContentType string
	Data        []byte
}

type messageContent struct {
	Body   string
	Images []messageImage
}
