package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/tenants"
)

// tenantFixture describe un tenant de prueba.
type tenantFixture struct {
	ID   int64
	Slug string
	Enc  []byte // nil = sin token (legacy)
}

func tenantsFixture(in ...tenantFixture) []tenants.Tenant {
	out := make([]tenants.Tenant, 0, len(in))
	for _, f := range in {
		out = append(out, tenants.Tenant{ID: f.ID, Slug: f.Slug, BotTokenEncrypted: f.Enc})
	}
	return out
}

// fakeBotServer emula la Bot API: getMe OK para cualquier token salvo
// los que contienen "bad" (401 → ErrInvalidToken); getUpdates responde
// lista vacia con pequena demora (evita busy-loop del poller en test).
func fakeBotServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			if strings.Contains(r.URL.Path, "bad") {
				fmt.Fprint(w, `{"ok":false,"description":"Unauthorized","error_code":401}`)
				return
			}
			fmt.Fprint(w, `{"ok":true,"result":{"id":99,"username":"fake_bot","first_name":"Fake"}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			time.Sleep(10 * time.Millisecond)
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{}}`)
	}))
}

// testRegistry construye un Registry contra el fake con decrypt
// identidad (el "cifrado" es el token en claro en tests).
func testRegistry(srv *httptest.Server) *Registry {
	r := NewRegistry(nil, nil)
	base := srv.URL
	r.newAdapter = func(token string) *Adapter {
		return NewAdapter(token, WithBaseURL(base))
	}
	return r
}

type sliceBus struct {
	n int
}

func (b *sliceBus) Publish(_ *Update) { b.n++ }

func TestRegistry_BootAll_ActiveAndDegraded(t *testing.T) {
	srv := fakeBotServer()
	defer srv.Close()
	r := testRegistry(srv)

	decrypt := func(blob []byte) (string, error) { return string(blob), nil }
	list := tenantsFixture(
		tenantFixture{ID: 1, Slug: "good", Enc: []byte("good-token")},
		tenantFixture{ID: 2, Slug: "bad", Enc: []byte("bad-token")},
		tenantFixture{ID: 3, Slug: "sin-token"},
	)
	buses := map[int64]*sliceBus{1: {}, 2: {}, 3: {}}
	r.BootAll(context.Background(), list, decrypt, func(id int64) Publisher {
		return buses[id]
	})

	// Tenant bueno: adapter + activo.
	if _, ok := r.AdapterFor(1); !ok {
		t.Error("tenant bueno sin adapter tras BootAll")
	}
	if st, _ := r.Status(1); st != "active" {
		t.Errorf("status bueno = %q, want active", st)
	}
	// Token revocado: degraded, SIN tumbar al bueno.
	if _, ok := r.AdapterFor(2); !ok {
		t.Error("tenant degradado sin adapter (debe existir para backoff)")
	}
	if st, _ := r.Status(2); st != "degraded" {
		t.Errorf("status revocado = %q, want degraded", st)
	}
	// Sin token: ni runtime.
	if _, ok := r.AdapterFor(3); ok {
		t.Error("tenant sin token tiene runtime: debe saltarse")
	}
	r.StopAll()
}

func TestRegistry_RegisterHot_Replaces(t *testing.T) {
	srv := fakeBotServer()
	defer srv.Close()
	r := testRegistry(srv)
	bus := &sliceBus{}

	r.RegisterHot(context.Background(), 7, "acme", "good-token", bus)
	first, ok := r.AdapterFor(7)
	if !ok {
		t.Fatal("sin adapter tras RegisterHot")
	}
	r.RegisterHot(context.Background(), 7, "acme", "good-token", bus)
	second, ok := r.AdapterFor(7)
	if !ok {
		t.Fatal("sin adapter tras segundo RegisterHot")
	}
	if first == second {
		t.Error("segundo RegisterHot no reemplazo el runtime")
	}
	if st, _ := r.Status(7); st != "active" {
		t.Errorf("status = %q, want active", st)
	}
	r.StopAll()
}

func TestRegistry_AdapterFor_Missing(t *testing.T) {
	r := NewRegistry(nil, nil)
	if _, ok := r.AdapterFor(999); ok {
		t.Error("AdapterFor(inexistente) = true, want false")
	}
	if _, ok := r.Status(999); ok {
		t.Error("Status(inexistente) = true, want false")
	}
}
