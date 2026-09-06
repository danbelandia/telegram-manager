package telegram

// Update es el objeto que Telegram entrega en getUpdates y en el
// webhook. Solo se modelan los subtipos que el MVP procesa (ver
// MVPAllowedUpdates); el resto llegan como nil y Kind() devuelve
// "unknown".
type Update struct {
	UpdateID        int64              `json:"update_id"`
	Message         *Message           `json:"message,omitempty"`
	ChatMember      *ChatMemberUpdated `json:"chat_member,omitempty"`
	MyChatMember    *ChatMemberUpdated `json:"my_chat_member,omitempty"`
	ChatJoinRequest *ChatJoinRequest   `json:"chat_join_request,omitempty"`
}

// Message modela un mensaje de chat. Campos minimos para moderacion
// (CD: borrar/fijar) y apertura/cierre por comandos.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text,omitempty"`
}

// User es un participante de Telegram.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat identifica un chat (privado, grupo, supergrupo, canal).
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}

// ChatMemberUpdated llega en updates chat_member y my_chat_member
// (cambios de estado de un miembro; ver ChatMemberStatus).
type ChatMemberUpdated struct {
	From          User       `json:"from"`
	NewChatMember ChatMember `json:"new_chat_member"`
	Date          int64      `json:"date"`
}

// ChatMember es el estado de un miembro dentro de un chat.
type ChatMember struct {
	Status string `json:"status"`
	User   *User  `json:"user,omitempty"`
}

// Estados de ChatMember.Status segun la Bot API.
const (
	MemberStatusCreator       = "creator"
	MemberStatusAdministrator = "administrator"
	MemberStatusMember        = "member"
	MemberStatusRestricted    = "restricted"
	MemberStatusLeft          = "left"
	MemberStatusKicked        = "kicked"
)

// ChatJoinRequest modela una solicitud de ingreso (ver telefono, el
// bot debe ser administrador y el grupo tener el modo de aprobacion).
type ChatJoinRequest struct {
	User User  `json:"user"`
	Chat Chat  `json:"chat"`
	Date int64 `json:"date"`
}

// Kind clasifica el update segun el subtipo presente, para logging y
// ruteo futuro del bus.
func (u *Update) Kind() string {
	switch {
	case u == nil:
		return "unknown"
	case u.Message != nil:
		return "message"
	case u.ChatMember != nil:
		return "chat_member"
	case u.MyChatMember != nil:
		return "my_chat_member"
	case u.ChatJoinRequest != nil:
		return "chat_join_request"
	default:
		return "unknown"
	}
}
