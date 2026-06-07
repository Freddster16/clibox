package app

type message struct {
	ID      int
	From    string
	Email   string
	Subject string
	Date    string
	Preview string
	Body    string
	Unread  bool
}

func fakeMessages() []message {
	return []message{
		{
			ID:      1,
			From:    "Alice",
			Email:   "alice@example.com",
			Subject: "Re: Design notes",
			Date:    "10:34 AM",
			Preview: "I looked at the prototype and left notes on the interaction pass.",
			Body: `Hey Freddy,

I looked at the prototype and left notes on the interaction pass.

The main thing I would tighten is the first-run path. If the app opens directly
to a useful inbox and keeps the key hints visible, I think people will trust it
faster.

Also: the reply-in-editor flow is the right call. It makes the whole thing feel
like it belongs in a coding session instead of trying to recreate a desktop mail
client in a terminal.

Talk soon,
Alice`,
			Unread: true,
		},
		{
			ID:      2,
			From:    "GitHub",
			Email:   "notifications@github.com",
			Subject: "New issue assigned",
			Date:    "Yesterday",
			Preview: "You were assigned issue #42 in Freddster16/clibox.",
			Body: `You were assigned issue #42 in Freddster16/clibox.

Title: Add Himalaya envelope adapter

The next implementation phase should replace the fake inbox data with a small
adapter that shells out to Himalaya and parses envelope JSON. Keep the UI model
independent from command spelling so Himalaya version changes stay contained.`,
			Unread: true,
		},
		{
			ID:      3,
			From:    "Vercel",
			Email:   "noreply@vercel.com",
			Subject: "Deployment failed",
			Date:    "Yesterday",
			Preview: "The preview deployment failed while building the project.",
			Body: `The preview deployment failed.

Build command:

  go test ./...

The failure is from a missing module dependency. Run go mod tidy locally and
push the generated go.sum file with the next change.`,
		},
		{
			ID:      4,
			From:    "Mom",
			Email:   "mom@example.com",
			Subject: "Dinner",
			Date:    "Jun 6",
			Preview: "Dinner is at 7. Bring the good stories.",
			Body: `Dinner is at 7.

Bring the good stories. Also, please do not spend the whole meal explaining why
email is better in a terminal unless someone asks first.`,
		},
		{
			ID:      5,
			From:    "Himalaya",
			Email:   "pimalaya@example.org",
			Subject: "Backend adapter notes",
			Date:    "Jun 5",
			Preview: "Keep command details behind one adapter boundary.",
			Body: `Backend adapter notes:

- Check whether Himalaya exists before trying to read mail.
- Parse JSON where available.
- Convert backend errors into short UI messages.
- Keep exact command syntax out of the Bubble Tea update loop.`,
		},
	}
}
