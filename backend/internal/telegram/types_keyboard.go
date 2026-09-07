package telegram

// InlineKeyboardButton y InlineKeyboardMarkup son los tipos publicos
// para construir el `reply_markup` que aceptan sendMessage y sendPhoto
// de la Bot API (docs/telegram_api_reference.md §inlinekeyboardmarkup).
// Los tags JSON coinciden 1:1 con los campos esperados por la API:
// `text` y `url` por boton, `inline_keyboard` como array de filas.
//
// El callback_data NO esta implementado: el panel solo publica, no
// recibe callback_query updates. Queda como trabajo futuro (slice 3
// / fase 4 de automatizaciones — design D3, exploration D3).

// InlineKeyboardButton representa un boton dentro de una fila del
// inline keyboard. `Text` es la etiqueta visible; `URL` es la
// URL HTTP o tg:// a abrir al presionar el boton.
type InlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// InlineKeyboardMarkup es la estructura completa: `InlineKeyboard` es
// un array de filas; cada fila es un array de botones (max 8 botones
// por fila, max 8 filas — limite autoimpuesto, no de la Bot API).
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
