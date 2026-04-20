package app

type Contact struct {
	ID           string
	Alias        string
	SignPubB64   string
	Mailbox      string
	PreKeyPubB64 string
}

type Message struct {
	EventID        string
	ChatID         string
	CreatedAt      int64
	SenderPubB64   string
	Text           string
	Direction      string
	PayloadRef     string
	PayloadHashB64 string
}
