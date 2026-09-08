// Templates de warning visual al usuario (Fase 3, slice 2.1, REQ-24/25).
// El paquete hardcodea 2 defaults (pre-mute y pre-ban) en español
// Rioplatense y expone RenderTemplate para substituir variables
// ({nombre}, {count}, {mute_minutes}) sobre un template custom por
// grupo o el default correspondiente. Cualquier error de substitucion o
// template vacio cae al default (best-effort, nunca aborta el pipeline).
//
// Invariante (bugfix #172): este paquete NO decide si se envia el
// warning (eso vive en Service.HandleMessage paso 7.5 via
// thresholdKindFor); solo formatea texto. El check de admin vive en el
// paquete automation.permissionOkAdmin.
package automation

import (
	"strings"

	"github.com/telegram-manager/backend/internal/telegram"
)

// TemplateKind clasifica el template a renderizar. Coincide 1:1 con
// WarningKind (pre_mute / pre_ban) — viven en types distintos para
// evitar acoplar el paquete templates con el wrapper de Telegram
// (templates podria testearse sin telegram.Service).
type TemplateKind string

const (
	// TemplatePreMute: warning antes de que el bot silencia al usuario.
	TemplatePreMute TemplateKind = "pre_mute"
	// TemplatePreBan: warning antes de que el bot banee al usuario.
	TemplatePreBan TemplateKind = "pre_ban"
)

// defaultTemplates son las 2 plantillas hardcoded que el bot envia
// cuando el grupo no customiza (warn_user_template IS NULL en la DB).
// Espanol Rioplatense (voseo), mismo registro que el resto del
// proyecto. Editables aca — NO son configurables runtime.
var defaultTemplates = map[TemplateKind]string{
	TemplatePreMute: "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min.",
	TemplatePreBan:  "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo.",
}

// DefaultTemplate devuelve el template default para un kind (util en
// tests y en la UI como placeholder del Textarea).
func DefaultTemplate(kind TemplateKind) string {
	return defaultTemplates[kind]
}

// MessageContext agrupa los datos del mensaje que el template puede
// necesitar. Vive en automation (no en telegram) para no acoplar el
// paquete con el wrapper del adapter; RenderTemplate usa solo los
// campos basicos (FirstName, Username) que ya son strings.
type MessageContext struct {
	FirstName string
	Username  string
}

// renderName aplica la fallback chain de {nombre}: FirstName si no
// vacio → Username sin el "@" si no vacio → literal "este usuario".
// Es la unica substitucion que NO es mecanica (los demas son
// format(strings)).
func renderName(msg *telegram.Message) string {
	if msg == nil || msg.From == nil {
		return "este usuario"
	}
	if msg.From.FirstName != "" {
		return msg.From.FirstName
	}
	if msg.From.Username != "" {
		// Quitar el "@" inicial si existe (Telegram suele mandarlo
		// sin el "@" en updates, pero si llega con el prefijo lo
		// removemos para presentacion consistente).
		return strings.TrimPrefix(msg.From.Username, "@")
	}
	return "este usuario"
}

// RenderTemplate produce el texto final del warning. Logica:
//
//  1. Si custom no es nil y NO esta vacio → usarlo.
//  2. Sino → usar el default del kind.
//  3. Substituir {nombre}, {count}, {mute_minutes} (orden no
//     importa: el orden de las substituciones evita pisarse entre si
//     porque los placeholders no se solapan).
//  4. Placeholders desconocidos se preservan verbatim (REQ-25).
//  5. Si el template custom fallo por estar vacio despues de trim →
//     cae al default. Log warn (lo emite el caller).
//
// Es best-effort: nunca retorna error. Si count <= 0 o settings nil,
// los placeholders quedan verbatim (defensivo).
func RenderTemplate(kind TemplateKind, settings *Settings, msg *telegram.Message, count int16, custom *string) string {
	tpl := defaultTemplates[kind]
	if custom != nil {
		trimmed := strings.TrimSpace(*custom)
		if trimmed != "" {
			tpl = trimmed
		}
		// Si custom es "" o solo whitespace, cae al default silenciosamente.
	}

	// Sustituciones. Usamos strings.Replace en lugar de fmt.Sprintf
	// para preservar placeholders desconocidos verbatim (REQ-25).
	out := tpl
	out = strings.ReplaceAll(out, "{nombre}", renderName(msg))

	if count > 0 {
		out = strings.ReplaceAll(out, "{count}", itoa(int(count)))
	}
	if settings != nil && settings.AutomuteMinutes > 0 {
		out = strings.ReplaceAll(out, "{mute_minutes}", itoa(int(settings.AutomuteMinutes)))
	}
	return out
}

// itoa es un helper minimalista para no importar strconv solo para
// int→string. Solo se usa con count y AutomuteMinutes (positivos,
// SMALLINT).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
