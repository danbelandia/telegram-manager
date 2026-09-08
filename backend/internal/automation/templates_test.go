// Tests de los templates de warning (Fase 3, slice 2.1, REQ-24/25).
// Cubre: fallback chain {nombre}, substitutions {count}/{mute_minutes},
// template custom vs default, empty → default, placeholder
// desconocido verbatim.
package automation

import (
	"strings"
	"testing"

	"github.com/telegram-manager/backend/internal/telegram"
)

// makeMsg construye un *telegram.Message minimo con el From pedido.
func makeMsg(firstName, username string) *telegram.Message {
	return &telegram.Message{
		MessageID: 1,
		From:      &telegram.User{ID: 999, FirstName: firstName, Username: username},
		Chat:      telegram.Chat{ID: -1001, Type: "supergroup"},
		Text:      "hi",
	}
}

func makeSettings(muteMin int16) *Settings {
	return &Settings{GroupID: -1001, AutomuteMinutes: muteMin}
}

// TestRenderTemplate_FirstName: {nombre} se sustituye por FirstName
// del From.
func TestRenderTemplate_FirstName(t *testing.T) {
	msg := makeMsg("Juan", "")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, nil)
	if !strings.Contains(out, "Juan") {
		t.Errorf("output sin FirstName: %q", out)
	}
	if strings.Contains(out, "{nombre}") {
		t.Errorf("{nombre} no se sustituyo: %q", out)
	}
}

// TestRenderTemplate_UsernameFallback: FirstName vacio → cae a
// Username sin el "@".
func TestRenderTemplate_UsernameFallback(t *testing.T) {
	msg := makeMsg("", "juanperez")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, nil)
	if !strings.Contains(out, "juanperez") {
		t.Errorf("Username no presente: %q", out)
	}
	if strings.Contains(out, "@") {
		t.Errorf("Username conserva el @: %q", out)
	}
}

// TestRenderTemplate_UsernameWithAtPrefix: si por algun motivo el
// username viene con "@", el sender lo limpia.
func TestRenderTemplate_UsernameWithAtPrefix(t *testing.T) {
	msg := makeMsg("", "@juanperez")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, nil)
	if strings.Contains(out, "@") {
		t.Errorf("@ no se removio: %q", out)
	}
	if !strings.Contains(out, "juanperez") {
		t.Errorf("username missing: %q", out)
	}
}

// TestRenderTemplate_LiteralFallback: FirstName vacio + Username
// vacio → literal "este usuario".
func TestRenderTemplate_LiteralFallback(t *testing.T) {
	msg := makeMsg("", "")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, nil)
	if !strings.Contains(out, "este usuario") {
		t.Errorf("fallback literal ausente: %q", out)
	}
}

// TestRenderTemplate_NilFrom: msg.From nil → cae al literal sin panic.
func TestRenderTemplate_NilFrom(t *testing.T) {
	msg := &telegram.Message{MessageID: 1, Chat: telegram.Chat{ID: -1001}}
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, nil)
	if !strings.Contains(out, "este usuario") {
		t.Errorf("fallback para From nil: %q", out)
	}
}

// TestRenderTemplate_Count: {count} se sustituye por el counter.
func TestRenderTemplate_Count(t *testing.T) {
	msg := makeMsg("Ana", "")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 7, nil)
	if !strings.Contains(out, "7") {
		t.Errorf("{count} no se sustituyo: %q", out)
	}
	if strings.Contains(out, "{count}") {
		t.Errorf("placeholder literal: %q", out)
	}
}

// TestRenderTemplate_MuteMinutes: {mute_minutes} se sustituye por
// AutomuteMinutes del setting.
func TestRenderTemplate_MuteMinutes(t *testing.T) {
	msg := makeMsg("Ana", "")
	out := RenderTemplate(TemplatePreMute, makeSettings(15), msg, 2, nil)
	if !strings.Contains(out, "15 min") {
		t.Errorf("{mute_minutes} no se sustituyo: %q", out)
	}
}

// TestRenderTemplate_CustomTemplate: si custom != nil, se usa el
// template custom (sustituciones siguen funcionando).
func TestRenderTemplate_CustomTemplate(t *testing.T) {
	msg := makeMsg("Pedro", "")
	custom := "🚨 {nombre} tiene {count} strikes. Cuidado."
	out := RenderTemplate(TemplatePreBan, makeSettings(10), msg, 3, &custom)
	if !strings.Contains(out, "Pedro") {
		t.Errorf("FirstName no sustituido: %q", out)
	}
	if !strings.Contains(out, "3 strikes") {
		t.Errorf("{count} no sustituido: %q", out)
	}
	if !strings.Contains(out, "🚨") {
		t.Errorf("emoji custom perdido: %q", out)
	}
	if strings.Contains(out, "{nombre}") || strings.Contains(out, "{count}") {
		t.Errorf("placeholder literal: %q", out)
	}
}

// TestRenderTemplate_EmptyCustomFallsBack: custom no-nil pero vacio o
// solo whitespace → cae al default.
func TestRenderTemplate_EmptyCustomFallsBack(t *testing.T) {
	msg := makeMsg("Ana", "")
	custom := "   "
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 2, &custom)
	// Debe matchear el default pre-mute (empieza con "⚠️").
	if !strings.HasPrefix(out, "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min.") {
		// Pero como los placeholders del default tambien se renderizan,
		// verificamos que contiene "serás silenciado" (unico del default pre-mute).
		if !strings.Contains(out, "serás silenciado") {
			t.Errorf("fallback al default no ocurrio: %q", out)
		}
	}
}

// TestRenderTemplate_UnknownPlaceholderVerbatim: placeholders que no
// conocemos se preservan literal (no se borran ni crashean).
func TestRenderTemplate_UnknownPlaceholderVerbatim(t *testing.T) {
	msg := makeMsg("Ana", "")
	custom := "Hola {nombre}, tu token es {token_xyz} y llevás {count}."
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 4, &custom)
	if !strings.Contains(out, "{token_xyz}") {
		t.Errorf("placeholder desconocido fue removido: %q", out)
	}
	if !strings.Contains(out, "Ana") {
		t.Errorf("FirstName no sustituido: %q", out)
	}
	if !strings.Contains(out, "4") {
		t.Errorf("{count} no sustituido: %q", out)
	}
}

// TestRenderTemplate_PreBanDefault: el default de pre-ban NO contiene
// {mute_minutes} → verificamos que sigue funcionando cuando el
// threshold es pre-ban.
func TestRenderTemplate_PreBanDefault(t *testing.T) {
	msg := makeMsg("Ana", "")
	out := RenderTemplate(TemplatePreBan, makeSettings(10), msg, 4, nil)
	if !strings.Contains(out, "serás expulsado") {
		t.Errorf("default pre-ban ausente: %q", out)
	}
	if strings.Contains(out, "{mute_minutes}") {
		t.Errorf("placeholder literal pre-ban: %q", out)
	}
	if !strings.Contains(out, "4") {
		t.Errorf("count no presente: %q", out)
	}
}

// TestRenderTemplate_ZeroCountLeavesPlaceholder: count == 0 deja
// {count} verbatim (defensivo — el caller deberia filtrar count==0).
func TestRenderTemplate_ZeroCountLeavesPlaceholder(t *testing.T) {
	msg := makeMsg("Ana", "")
	out := RenderTemplate(TemplatePreMute, makeSettings(10), msg, 0, nil)
	if !strings.Contains(out, "{count}") {
		t.Errorf("count=0 deberia dejar placeholder: %q", out)
	}
}

// TestRenderTemplate_PreBanWithMuteMinutesInCustom: si el admin usa
// {mute_minutes} en un template de pre-ban, se sigue renderizando (es
// informativo; spec REQ-24).
func TestRenderTemplate_PreBanWithMuteMinutesInCustom(t *testing.T) {
	msg := makeMsg("Ana", "")
	custom := "{nombre} tu mute seria de {mute_minutes} min, pero en realidad serás expulsado."
	out := RenderTemplate(TemplatePreBan, makeSettings(7), msg, 4, &custom)
	if !strings.Contains(out, "Ana") {
		t.Errorf("nombre missing: %q", out)
	}
	if !strings.Contains(out, "7 min") {
		t.Errorf("mute_minutes missing: %q", out)
	}
	if !strings.Contains(out, "expulsado") {
		t.Errorf("texto custom missing: %q", out)
	}
}

// TestDefaultTemplate_NotEmpty: los 2 defaults existen y no son vacios
// (defensa contra errores de dedo).
func TestDefaultTemplate_NotEmpty(t *testing.T) {
	if DefaultTemplate(TemplatePreMute) == "" {
		t.Error("default pre-mute vacio")
	}
	if DefaultTemplate(TemplatePreBan) == "" {
		t.Error("default pre-ban vacio")
	}
	if !strings.Contains(DefaultTemplate(TemplatePreMute), "{nombre}") {
		t.Error("default pre-mute sin {nombre}")
	}
	if !strings.Contains(DefaultTemplate(TemplatePreMute), "{count}") {
		t.Error("default pre-mute sin {count}")
	}
	if !strings.Contains(DefaultTemplate(TemplatePreMute), "{mute_minutes}") {
		t.Error("default pre-mute sin {mute_minutes}")
	}
	if !strings.Contains(DefaultTemplate(TemplatePreBan), "{nombre}") {
		t.Error("default pre-ban sin {nombre}")
	}
}
