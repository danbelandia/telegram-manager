package publications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// testTenantID es el tenant de los servicios bajo test (slice 0). Los
// fakes en memoria SÍ filtran por tenant (como la DB): el aislamiento
// real entre tenants se prueba en repository_test con Postgres.
const testTenantID = 1

// fakeGroupsPub implementa GroupReader con un mapa en memoria.
type fakeGroupsPub struct {
	groups map[int64]*groups.Group
}

func (f *fakeGroupsPub) GetByTenant(_ context.Context, _ int64, id int64) (*groups.Group, error) {
	g, ok := f.groups[id]
	if !ok {
		return nil, groups.ErrNotFound
	}
	return g, nil
}

// fakeTelegramPub implementa MessageSender registrando cada llamada.
// Soporta SendMessage y SendPhoto (slice 2) — el `kind` indica que
// metodo se invoco, lo cual se usa en tests que verifican el orden
// o que diferencian texto vs foto.
type fakeTelegramPub struct {
	calls []fakeTelegramCall
	// sent conserva solo los envios de texto para retrocompatibilidad
	// con tests slice-1 que solo inspeccionaban SendMessage.
	sent []struct {
		chatID int64
		text   string
	}
	messageID int64 // returned by SendMessage/SendPhoto
	err       error
	// perCallErr, si esta set, hace que la llamada i-esima falle.
	// Es nil-indexed (la primera llamada al primer indice, etc.).
	// Solo se consulta cuando err es nil.
	perCallErr []error
}

// fakeTelegramCall registra una invocacion al adapter mockeado.
type fakeTelegramCall struct {
	kind     string // "message", "photo", "video", "photo_upload", "video_upload"
	chatID   int64
	text     string // text o caption segun kind
	photoURL string
}

func (f *fakeTelegramPub) SendMessage(_ context.Context, chatID int64, text string, _ bool, _ *telegram.InlineKeyboardMarkup) (int64, error) {
	f.calls = append(f.calls, fakeTelegramCall{kind: "message", chatID: chatID, text: text})
	f.sent = append(f.sent, struct {
		chatID int64
		text   string
	}{chatID, text})
	return f.sendResult(len(f.calls) - 1)
}

func (f *fakeTelegramPub) SendPhoto(_ context.Context, chatID int64, photoURL, caption string, _ *telegram.InlineKeyboardMarkup) (int64, error) {
	f.calls = append(f.calls, fakeTelegramCall{kind: "photo", chatID: chatID, text: caption, photoURL: photoURL})
	return f.sendResult(len(f.calls) - 1)
}

func (f *fakeTelegramPub) SendVideo(_ context.Context, chatID int64, videoURL, caption string, _ *telegram.InlineKeyboardMarkup) (int64, error) {
	f.calls = append(f.calls, fakeTelegramCall{kind: "video", chatID: chatID, text: caption, photoURL: videoURL})
	return f.sendResult(len(f.calls) - 1)
}

func (f *fakeTelegramPub) SendPhotoUpload(_ context.Context, chatID int64, _ io.Reader, _, caption string, _ *telegram.InlineKeyboardMarkup) (int64, error) {
	f.calls = append(f.calls, fakeTelegramCall{kind: "photo_upload", chatID: chatID, text: caption})
	return f.sendResult(len(f.calls) - 1)
}

func (f *fakeTelegramPub) SendVideoUpload(_ context.Context, chatID int64, _ io.Reader, _, caption string, _ *telegram.InlineKeyboardMarkup) (int64, error) {
	f.calls = append(f.calls, fakeTelegramCall{kind: "video_upload", chatID: chatID, text: caption})
	return f.sendResult(len(f.calls) - 1)
}

func (f *fakeTelegramPub) sendResult(idx int) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if idx < len(f.perCallErr) && f.perCallErr[idx] != nil {
		return 0, f.perCallErr[idx]
	}
	return f.messageID, nil
}

// fakeLogsPub implementa LogWriter capturando entries.
type fakeLogsPub struct {
	entries []*logs.Entry
}

func (f *fakeLogsPub) Create(ctx context.Context, e *logs.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

// fakePubStore implementa PubStore en memoria con ID autoincremental.
type fakePubStore struct {
	pubs   map[int64]*Publication
	nextID int64
}

func newFakePubStore() *fakePubStore {
	return &fakePubStore{pubs: make(map[int64]*Publication), nextID: 1}
}

func (f *fakePubStore) Create(ctx context.Context, p *Publication) error {
	p.ID = f.nextID
	f.nextID++
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	f.pubs[p.ID] = p
	return nil
}

func (f *fakePubStore) GetByID(_ context.Context, tenantID, id int64) (*Publication, error) {
	p, ok := f.pubs[id]
	if !ok || p.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakePubStore) List(_ context.Context, tenantID int64, limit, offset int) ([]Publication, error) {
	out := make([]Publication, 0, len(f.pubs))
	for _, p := range f.pubs {
		if p.TenantID != tenantID {
			continue
		}
		out = append(out, *p)
	}
	// Orden estable por created_at DESC para coincidir con la DB.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return paginate(out, limit, offset), nil
}

// ListByTelegramID filtra por tenant + telegram_id y pagina con limit/offset.
func (f *fakePubStore) ListByTelegramID(_ context.Context, tenantID, telegramID int64, limit, offset int) ([]Publication, error) {
	out := make([]Publication, 0)
	for _, p := range f.pubs {
		if p.TenantID != tenantID || p.TelegramID != telegramID {
			continue
		}
		out = append(out, *p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return paginate(out, limit, offset), nil
}

// ClaimScheduledDue (slice 3): replica el patron SKIP LOCKED devolviendo
// las filas `scheduled` cuya `scheduled_at <= now`. Marca `sending` en
// memoria. No hay locks reales (es un fake) pero el comportamiento es
// equivalente para los tests.
func (f *fakePubStore) ClaimScheduledDue(_ context.Context, tenantID int64, limit int) ([]Publication, error) {
	now := time.Now()
	claimed := make([]Publication, 0, limit)
	for _, p := range f.pubs {
		if p.TenantID != tenantID {
			continue
		}
		if p.Status != StatusScheduled || p.ScheduledAt == nil {
			continue
		}
		if p.ScheduledAt.After(now) {
			continue
		}
		if len(claimed) >= limit {
			break
		}
		p.Status = StatusSending
		p.UpdatedAt = now
		claimed = append(claimed, *p)
	}
	sort.SliceStable(claimed, func(i, j int) bool {
		return claimLess(claimed[i], claimed[j])
	})
	return claimed, nil
}

// Cancel (slice 3): hard delete SOLO si status == scheduled y el tenant
// coincide. Otros status -> ErrCancelNotAllowed. Inexistente o ajeno ->
// ErrNotFound.
func (f *fakePubStore) Cancel(_ context.Context, tenantID, id int64) error {
	p, ok := f.pubs[id]
	if !ok || p.TenantID != tenantID {
		return ErrNotFound
	}
	if p.Status != StatusScheduled {
		return ErrCancelNotAllowed
	}
	delete(f.pubs, id)
	return nil
}

// paginate aplica limit/offset sobre un slice ya ordenado.
func paginate(in []Publication, limit, offset int) []Publication {
	if offset >= len(in) {
		return []Publication{}
	}
	end := offset + limit
	if end > len(in) {
		end = len(in)
	}
	return in[offset:end]
}

// claimLess ordena filas por scheduled_at ASC con los punteros nil al
// final (no deberia haberlos en filas `scheduled`, pero es defensivo).
func claimLess(a, b Publication) bool {
	if a.ScheduledAt == nil {
		return false
	}
	if b.ScheduledAt == nil {
		return true
	}
	return a.ScheduledAt.Before(*b.ScheduledAt)
}

func (f *fakePubStore) UpdateStatus(_ context.Context, tenantID, id int64, status Status, messageID *int64, errMsg *string) error {
	p, ok := f.pubs[id]
	if !ok || p.TenantID != tenantID {
		return ErrNotFound
	}
	p.Status = status
	p.MessageID = messageID
	p.ErrorMessage = errMsg
	p.UpdatedAt = time.Now()
	return nil
}

// newPubService construye un Service de pruebas con fakes.
func newPubService(t *testing.T, tg *fakeTelegramPub, store *fakePubStore, groupsMap map[int64]*groups.Group) (*Service, *fakeLogsPub) {
	t.Helper()
	if store == nil {
		store = newFakePubStore()
	}
	logsFake := &fakeLogsPub{}
	svc := NewService(&fakeGroupsPub{groups: groupsMap}, tg, store, logsFake)
	return svc, logsFake
}

// --- Tests de Publish (single-group, slice 1 + slice 2 compatibilidad) ---

func TestService_PublishSuccess(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 123}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	pub, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Hola mundo", nil, nil)
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if pub.Status != StatusSent {
		t.Errorf("status = %s, want sent", pub.Status)
	}
	if pub.MessageID == nil || *pub.MessageID != 123 {
		t.Errorf("message_id = %v, want 123", pub.MessageID)
	}
	if len(tg.sent) != 1 || tg.sent[0].chatID != -1001 || tg.sent[0].text != "Hola mundo" {
		t.Errorf("sent = %v, want una llamada a SendMessage(-1001, 'Hola mundo')", tg.sent)
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(logFake.entries))
	}
	e := logFake.entries[0]
	if e.Action != logs.ActionPublishMessage || e.Status != logs.StatusSuccess {
		t.Errorf("log = %s/%s, want PUBLISH_MESSAGE/SUCCESS", e.Action, e.Status)
	}
	if e.Metadata["publication_id"] != pub.ID {
		t.Errorf("metadata publication_id = %v, want %d", e.Metadata["publication_id"], pub.ID)
	}
	if e.Metadata["message_id"] != int64(123) {
		t.Errorf("metadata message_id = %v, want 123", e.Metadata["message_id"])
	}
}

func TestService_PublishTextEmpty(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	_, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "", nil, nil)
	if !errors.Is(err, ErrTextEmpty) {
		t.Fatalf("Publish() error = %v, want ErrTextEmpty", err)
	}
	if len(tg.sent) != 0 {
		t.Errorf("se llamo a Telegram con texto vacio: %v", tg.sent)
	}
	if len(logFake.entries) != 0 {
		t.Errorf("logs = %d, want 0 (texto vacio, sin log)", len(logFake.entries))
	}
}

func TestService_PublishTextTooLong(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	longText := strings.Repeat("a", 4097)
	_, err := svc.Publish(context.Background(), testTenantID, 7, -1001, longText, nil, nil)
	if !errors.Is(err, ErrTextTooLong) {
		t.Fatalf("Publish() error = %v, want ErrTextTooLong", err)
	}
	if len(tg.sent) != 0 {
		t.Errorf("se llamo a Telegram con texto demasiado largo: %v", tg.sent)
	}
	if len(logFake.entries) != 0 {
		t.Errorf("logs = %d, want 0 (texto invalido, sin log)", len(logFake.entries))
	}
}

func TestService_PublishGroupNotFound(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{})

	_, err := svc.Publish(context.Background(), testTenantID, 7, -999, "Hola", nil, nil)
	if !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("Publish() error = %v, want ErrGroupNotFound", err)
	}
	if len(tg.sent) != 0 {
		t.Errorf("se llamo a Telegram con grupo inexistente: %v", tg.sent)
	}
	if len(logFake.entries) != 0 {
		t.Errorf("logs = %d, want 0 (sin grupo administrado que auditar)", len(logFake.entries))
	}
}

func TestService_PublishNoPermission(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember},
	})

	_, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Hola", nil, nil)
	if !errors.Is(err, ErrBotPermission) {
		t.Fatalf("Publish() error = %v, want ErrBotPermission", err)
	}
	if len(tg.sent) != 0 {
		t.Errorf("se llamo a Telegram sin permiso: %v", tg.sent)
	}
	if len(store.pubs) != 0 {
		t.Errorf("pub entries = %d, want 0 (no se persiste sin permiso)", len(store.pubs))
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1 (fallo registrado)", len(logFake.entries))
	}
	e := logFake.entries[0]
	if e.Status != logs.StatusPermissionDenied {
		t.Errorf("log status = %s, want PERMISSION_DENIED", e.Status)
	}
}

func TestService_PublishTelegramError(t *testing.T) {
	tg := &fakeTelegramPub{err: telegram.ErrPermissionDenied}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	_, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Hola", nil, nil)
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Fatalf("Publish() error = %v, want ErrPermissionDenied del adapter", err)
	}
	pub := store.pubs[1]
	if pub.Status != StatusFailed {
		t.Errorf("status = %s, want failed", pub.Status)
	}
	if pub.ErrorMessage == nil || *pub.ErrorMessage == "" {
		t.Error("error_message ausente en publicacion fallida")
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(logFake.entries))
	}
	if logFake.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("log status = %s, want PERMISSION_DENIED", logFake.entries[0].Status)
	}
}

func TestService_List(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	if _, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Primera", nil, nil); err != nil {
		t.Fatalf("Publish 1 error: %v", err)
	}
	if _, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Segunda", nil, nil); err != nil {
		t.Fatalf("Publish 2 error: %v", err)
	}

	list, err := svc.List(context.Background(), testTenantID, 50, 0)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestService_GetByID(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 55}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	pub, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "Hola", nil, nil)
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	got, err := svc.GetByID(context.Background(), testTenantID, pub.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.ID != pub.ID || got.Text != "Hola" {
		t.Errorf("got = %+v, want publicacion con texto 'Hola'", got)
	}

	_, err = svc.GetByID(context.Background(), testTenantID, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(999) error = %v, want ErrNotFound", err)
	}
}

// --- Tests de PublishMany (slice 2) ---

func TestService_PublishMany_OK_MultiGrupo(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 100}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
	})

	payload := PublishPayload{
		Text:     "Hola",
		GroupIDs: []int64{-1001, -1002},
	}
	rows, err := svc.PublishMany(context.Background(), testTenantID, 7, payload)
	if err != nil {
		t.Fatalf("PublishMany() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for i, row := range rows {
		if row.Status != StatusSent {
			t.Errorf("row[%d].status = %s, want sent", i, row.Status)
		}
		if row.MessageID == nil || *row.MessageID != 100 {
			t.Errorf("row[%d].message_id = %v, want 100", i, row.MessageID)
		}
	}
	if len(logFake.entries) != 2 {
		t.Errorf("logs = %d, want 2 (1 por grupo)", len(logFake.entries))
	}
}

func TestService_PublishMany_OrdenSecuencial(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
		-1003: {TelegramID: -1003, BotStatus: groups.StatusAdministrator},
	})

	payload := PublishPayload{Text: "Hola", GroupIDs: []int64{-1003, -1001, -1002}}
	if _, err := svc.PublishMany(context.Background(), testTenantID, 7, payload); err != nil {
		t.Fatalf("PublishMany() error: %v", err)
	}
	if len(tg.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(tg.calls))
	}
	want := []int64{-1003, -1001, -1002}
	for i, want := range want {
		if tg.calls[i].chatID != want {
			t.Errorf("calls[%d].chatID = %d, want %d (orden secuencial)", i, tg.calls[i].chatID, want)
		}
	}
}

func TestService_PublishMany_FalloParcial_Telegram(t *testing.T) {
	tg := &fakeTelegramPub{
		messageID: 1,
		perCallErr: []error{
			telegram.ErrPermissionDenied, // g1 falla
			nil,                          // g2 ok
		},
	}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
	})

	payload := PublishPayload{Text: "Hola", GroupIDs: []int64{-1001, -1002}}
	rows, err := svc.PublishMany(context.Background(), testTenantID, 7, payload)
	if err != nil {
		t.Fatalf("PublishMany() error: %v (esperabamos fallo parcial sin abortar)", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Status != StatusFailed {
		t.Errorf("rows[0].status = %s, want failed", rows[0].Status)
	}
	if rows[0].ErrorMessage == nil {
		t.Error("rows[0].error_message ausente")
	}
	if rows[1].Status != StatusSent {
		t.Errorf("rows[1].status = %s, want sent (g2 no abortado por fallo en g1)", rows[1].Status)
	}
	if len(logFake.entries) != 2 {
		t.Errorf("logs = %d, want 2", len(logFake.entries))
	}
	if logFake.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("log[0].status = %s, want PERMISSION_DENIED", logFake.entries[0].Status)
	}
	if logFake.entries[1].Status != logs.StatusSuccess {
		t.Errorf("log[1].status = %s, want SUCCESS", logFake.entries[1].Status)
	}
}

func TestService_PublishMany_FalloParcial_Permiso(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusMember}, // sin admin
	})

	payload := PublishPayload{Text: "Hola", GroupIDs: []int64{-1001, -1002}}
	rows, err := svc.PublishMany(context.Background(), testTenantID, 7, payload)
	if err != nil {
		t.Fatalf("PublishMany() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Status != StatusSent {
		t.Errorf("rows[0].status = %s, want sent", rows[0].Status)
	}
	if rows[1].Status != StatusFailed {
		t.Errorf("rows[1].status = %s, want failed", rows[1].Status)
	}
	if rows[1].ErrorMessage == nil || !strings.Contains(*rows[1].ErrorMessage, "administrador") {
		t.Errorf("rows[1].error_message = %v, want mensaje de permiso", rows[1].ErrorMessage)
	}
	// g1 no debe haber sido enviado a Telegram.
	if len(tg.calls) != 1 || tg.calls[0].chatID != -1001 {
		t.Errorf("solo g1 debio ser enviado a Telegram; calls = %v", tg.calls)
	}
	if len(logFake.entries) != 2 {
		t.Errorf("logs = %d, want 2", len(logFake.entries))
	}
}

func TestService_PublishMany_GrupoInexistente_NoAborta(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	payload := PublishPayload{Text: "Hola", GroupIDs: []int64{-1001, -9999}}
	rows, err := svc.PublishMany(context.Background(), testTenantID, 7, payload)
	if err != nil {
		t.Fatalf("PublishMany() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Status != StatusSent {
		t.Errorf("rows[0].status = %s, want sent", rows[0].Status)
	}
	if rows[1].Status != StatusFailed {
		t.Errorf("rows[1].status = %s, want failed (grupo no existe)", rows[1].Status)
	}
	// Solo 1 envio a Telegram.
	if len(tg.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(tg.calls))
	}
	// 2 logs: 1 success + 1 NOT_FOUND.
	if len(logFake.entries) != 2 {
		t.Errorf("logs = %d, want 2", len(logFake.entries))
	}
	if logFake.entries[1].Status != logs.StatusNotFound {
		t.Errorf("log[1].status = %s, want NOT_FOUND", logFake.entries[1].Status)
	}
}

func TestService_PublishMany_ConFotoYBotones(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	photo := "https://example.com/x.jpg"
	buttons := [][]telegram.InlineKeyboardButton{
		{{Text: "Ir", URL: "https://example.com"}},
	}
	payload := PublishPayload{
		Text:     "Con foto",
		PhotoURL: &photo,
		Buttons:  buttons,
		GroupIDs: []int64{-1001},
	}
	rows, err := svc.PublishMany(context.Background(), testTenantID, 7, payload)
	if err != nil {
		t.Fatalf("PublishMany() error: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != StatusSent {
		t.Fatalf("rows = %+v, want 1 sent", rows)
	}
	if len(tg.calls) != 1 || tg.calls[0].kind != "photo" {
		t.Fatalf("calls = %v, want 1 photo call", tg.calls)
	}
	if tg.calls[0].photoURL != photo || tg.calls[0].text != "Con foto" {
		t.Errorf("call[0] = %+v, want photo/caption correctos", tg.calls[0])
	}
	if rows[0].PhotoURL == nil || *rows[0].PhotoURL != photo {
		t.Errorf("rows[0].photo_url = %v, want %s", rows[0].PhotoURL, photo)
	}
	if len(rows[0].Buttons) == 0 {
		t.Errorf("rows[0].buttons vacio")
	}
}

// --- Tests de validacion fail-fast (validatePayload) ---

func TestService_PublishMany_Validacion_TextVacio(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "",
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrTextEmpty) {
		t.Fatalf("error = %v, want ErrTextEmpty", err)
	}
	if len(tg.calls) != 0 {
		t.Errorf("se llamo a Telegram pese a texto vacio: %v", tg.calls)
	}
}

func TestService_PublishMany_Validacion_CaptionExcede1024(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	photo := "https://example.com/x.jpg"
	longCaption := strings.Repeat("a", 1025)
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     longCaption,
		PhotoURL: &photo,
		GroupIDs: []int64{-1001},
	})
	if err == nil || !strings.Contains(err.Error(), "1024") {
		t.Fatalf("error = %v, want mensaje sobre 1024 chars", err)
	}
}

func TestService_PublishMany_Validacion_TextExcede4096(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	longText := strings.Repeat("a", 4097)
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     longText,
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrTextTooLong) {
		t.Fatalf("error = %v, want ErrTextTooLong", err)
	}
}

func TestService_PublishMany_Validacion_PhotoURLNoHTTP(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	bad := "ftp://example.com/x.jpg"
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		PhotoURL: &bad,
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrPhotoURLScheme) {
		t.Fatalf("error = %v, want ErrPhotoURLScheme", err)
	}
}

func TestService_PublishMany_Validacion_PhotoURLMuyLarga(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	bad := "https://example.com/" + strings.Repeat("a", 2048)
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		PhotoURL: &bad,
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrPhotoURLTooLong) {
		t.Fatalf("error = %v, want ErrPhotoURLTooLong", err)
	}
}

func TestService_PublishMany_Validacion_BotonesMas8Filas(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	rows := make([][]telegram.InlineKeyboardButton, 9)
	for i := range rows {
		rows[i] = []telegram.InlineKeyboardButton{{Text: "A", URL: "https://a"}}
	}
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		Buttons:  rows,
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrButtonsLimit) {
		t.Fatalf("error = %v, want ErrButtonsLimit", err)
	}
}

func TestService_PublishMany_Validacion_BotonURLNoHTTP(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	rows := [][]telegram.InlineKeyboardButton{
		{{Text: "XSS", URL: "javascript:alert(1)"}},
	}
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		Buttons:  rows,
		GroupIDs: []int64{-1001},
	})
	if !errors.Is(err, ErrButtonURLScheme) {
		t.Fatalf("error = %v, want ErrButtonURLScheme", err)
	}
}

func TestService_PublishMany_Validacion_GruposVacio(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		GroupIDs: []int64{},
	})
	if !errors.Is(err, ErrGroupsEmpty) {
		t.Fatalf("error = %v, want ErrGroupsEmpty", err)
	}
}

func TestService_PublishMany_Validacion_GruposExcede10(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	ids := make([]int64, 11)
	for i := range ids {
		ids[i] = int64(-1000 - i)
	}
	_, err := svc.PublishMany(context.Background(), testTenantID, 7, PublishPayload{
		Text:     "hola",
		GroupIDs: ids,
	})
	if !errors.Is(err, ErrGroupsLimit) {
		t.Fatalf("error = %v, want ErrGroupsLimit", err)
	}
}

func TestService_ListByTelegramID_Filtra(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
	})

	if _, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "a", nil, nil); err != nil {
		t.Fatalf("Publish g1: %v", err)
	}
	if _, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "b", nil, nil); err != nil {
		t.Fatalf("Publish g1: %v", err)
	}
	if _, err := svc.Publish(context.Background(), testTenantID, 7, -1002, "c", nil, nil); err != nil {
		t.Fatalf("Publish g2: %v", err)
	}

	g1, err := svc.ListByTelegramID(context.Background(), testTenantID, -1001, 50, 0)
	if err != nil {
		t.Fatalf("ListByTelegramID: %v", err)
	}
	if len(g1) != 2 {
		t.Errorf("g1 len = %d, want 2", len(g1))
	}
	for _, p := range g1 {
		if p.TelegramID != -1001 {
			t.Errorf("publicacion con telegram_id=%d en grupo -1001", p.TelegramID)
		}
	}

	g2, err := svc.ListByTelegramID(context.Background(), testTenantID, -1002, 50, 0)
	if err != nil {
		t.Fatalf("ListByTelegramID: %v", err)
	}
	if len(g2) != 1 {
		t.Errorf("g2 len = %d, want 1", len(g2))
	}

	empty, err := svc.ListByTelegramID(context.Background(), testTenantID, -9999, 50, 0)
	if err != nil {
		t.Fatalf("ListByTelegramID: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty len = %d, want 0", len(empty))
	}
}

// TestService_PublishMany_MarshalButtons_RoundTrip: confirma que
// los bytes almacenados son deserializables al mismo shape.
func TestService_PublishMany_MarshalButtons_RoundTrip(t *testing.T) {
	raw, err := MarshalButtons([][]telegram.InlineKeyboardButton{
		{{Text: "Ir", URL: "https://a"}},
		{{Text: "B", URL: "https://b"}, {Text: "C", URL: "https://c"}},
	})
	if err != nil {
		t.Fatalf("MarshalButtons: %v", err)
	}
	got, err := UnmarshalButtons(raw)
	if err != nil {
		t.Fatalf("UnmarshalButtons: %v", err)
	}
	if len(got) != 2 || len(got[0]) != 1 || len(got[1]) != 2 {
		t.Fatalf("shape = %v, want 2 filas (1 + 2 botones)", got)
	}
	// Re-marshal y comparar.
	raw2, _ := MarshalButtons(got)
	if !jsonEqual(raw, raw2) {
		t.Errorf("round-trip cambia shape:\n  in:  %s\n  out: %s", raw, raw2)
	}
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &y); err != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	ay, _ := json.Marshal(y)
	return string(ax) == string(ay)
}

// --- Tests de slice 3: Schedule + CancelScheduled ---

// TestService_Schedule_InsertsScheduled_SinLlamarTelegram: la rama de
// scheduling inserta N filas con status=scheduled y NO invoca SendMessage/SendPhoto.
func TestService_Schedule_InsertsScheduled_SinLlamarTelegram(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
	})

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	scheduledAt := now.Add(2 * time.Hour)

	rows, err := svc.Schedule(context.Background(), testTenantID, 7,
		PublishPayload{Text: "Hola", GroupIDs: []int64{-1001, -1002}},
		scheduledAt, fixedNow(now))
	if err != nil {
		t.Fatalf("Schedule() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for i, r := range rows {
		if r.Status != StatusScheduled {
			t.Errorf("rows[%d].status = %s, want scheduled", i, r.Status)
		}
		if r.ScheduledAt == nil || !r.ScheduledAt.Equal(scheduledAt) {
			t.Errorf("rows[%d].scheduled_at = %v, want %v", i, r.ScheduledAt, scheduledAt)
		}
	}
	// CRITICO: Telegram NO se llama en la rama schedule.
	if len(tg.calls) != 0 {
		t.Errorf("tg.calls = %d, want 0 (Schedule no debe llamar Telegram)", len(tg.calls))
	}
	if len(tg.sent) != 0 {
		t.Errorf("tg.sent = %d, want 0", len(tg.sent))
	}
	// Sin logs: la auditoria la emite el worker cuando procesa cada fila.
	if len(logFake.entries) != 0 {
		t.Errorf("logs = %d, want 0 (auditoria es del worker)", len(logFake.entries))
	}
	// store tiene las filas persistidas.
	if len(store.pubs) != 2 {
		t.Errorf("store.pubs = %d, want 2", len(store.pubs))
	}
}

// TestService_Schedule_PastDate_ErrScheduledInPast: validar fecha pasada
// es fail-fast: ni inserta ni llama Telegram.
func TestService_Schedule_PastDate_ErrScheduledInPast(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	_, err := svc.Schedule(context.Background(), testTenantID, 7,
		PublishPayload{Text: "Hola", GroupIDs: []int64{-1001}},
		now.Add(-time.Minute), fixedNow(now))
	if !errors.Is(err, ErrScheduledInPast) {
		t.Fatalf("error = %v, want ErrScheduledInPast", err)
	}
	if len(tg.calls) != 0 {
		t.Errorf("tg.calls = %d, want 0", len(tg.calls))
	}
	if len(store.pubs) != 0 {
		t.Errorf("store.pubs = %d, want 0 (no se persiste)", len(store.pubs))
	}
}

// TestService_Schedule_EqualNow_ErrScheduledInPast: tolerancia cero,
// igual a now() tambien es pasado.
func TestService_Schedule_EqualNow_ErrScheduledInPast(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	_, err := svc.Schedule(context.Background(), testTenantID, 7,
		PublishPayload{Text: "x", GroupIDs: []int64{-1001}},
		now, fixedNow(now))
	if !errors.Is(err, ErrScheduledInPast) {
		t.Errorf("error = %v, want ErrScheduledInPast (tolerancia cero)", err)
	}
}

// TestService_Schedule_Validation_TextEmpty: el fail-fast de payload
// corre ANTES de validar scheduled_at.
func TestService_Schedule_Validation_TextEmpty(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)

	now := time.Now()
	_, err := svc.Schedule(context.Background(), testTenantID, 7,
		PublishPayload{Text: "", GroupIDs: []int64{-1001}},
		now.Add(time.Hour), fixedNow(now))
	if !errors.Is(err, ErrTextEmpty) {
		t.Errorf("error = %v, want ErrTextEmpty", err)
	}
}

// TestService_CancelScheduled_OK: borra la fila cuando status=scheduled.
func TestService_CancelScheduled_OK(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, nil)

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	rows, err := svc.Schedule(context.Background(), testTenantID, 7,
		PublishPayload{Text: "x", GroupIDs: []int64{-1001}},
		now.Add(time.Hour), fixedNow(now))
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	id := rows[0].ID

	if err := svc.CancelScheduled(context.Background(), testTenantID, id); err != nil {
		t.Errorf("CancelScheduled error = %v, want nil", err)
	}
	if _, ok := store.pubs[id]; ok {
		t.Errorf("fila %d sigue en store, want borrada", id)
	}
}

// TestService_CancelScheduled_Sent_ReturnsErrCancelNotAllowed: no se
// puede cancelar una fila ya enviada (preserva audit trail).
func TestService_CancelScheduled_Sent_ReturnsErrCancelNotAllowed(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	pub, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "hola", nil, nil)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.Status != StatusSent {
		t.Fatalf("setup: pub.Status = %s, want sent", pub.Status)
	}
	if err := svc.CancelScheduled(context.Background(), testTenantID, pub.ID); !errors.Is(err, ErrCancelNotAllowed) {
		t.Errorf("error = %v, want ErrCancelNotAllowed", err)
	}
	// La fila sigue en el store.
	if _, ok := store.pubs[pub.ID]; !ok {
		t.Errorf("fila %d borrada, want intacta", pub.ID)
	}
}

// TestService_CancelScheduled_NotFound: id inexistente -> ErrNotFound.
func TestService_CancelScheduled_NotFound(t *testing.T) {
	tg := &fakeTelegramPub{}
	svc, _ := newPubService(t, tg, newFakePubStore(), nil)
	if err := svc.CancelScheduled(context.Background(), testTenantID, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestService_List_PropagatesLimitOffset: limit/offset llegan al store.
func TestService_List_PropagatesLimitOffset(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})
	for i := 0; i < 5; i++ {
		if _, err := svc.Publish(context.Background(), testTenantID, 7, -1001, "x", nil, nil); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
	}
	list, err := svc.List(context.Background(), testTenantID, 2, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}
