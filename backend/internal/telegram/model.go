package telegram

// BotUser es la identidad del bot devuelta por getMe.
type BotUser struct {
	ID        int64
	Username  string
	FirstName string
}
