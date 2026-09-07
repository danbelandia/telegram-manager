package publications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakeGroupsPub implementa GroupReader con un mapa en memoria.
type fakeGroupsPub struct {
	groups map[int64]*groups.Group
}

func (f *fakeGroupsPub) GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error) {
	g, ok := f.groups[id]
	if !ok {
		return nil, groups.ErrNotFound
	}
	return g, nil
}

// fakeTelegramPub implementa MessageSender registrando cada llamada.
type fakeTelegramPub struct {
	sent []struct {
		chatID int64
		text   string
	}
	messageID int64 // returned by SendMessage
	err       error
}

func (f *fakeTelegramPub) SendMessage(_ context.Context, chatID int64, text string, _ bool) (int64, error) {
	f.sent = append(f.sent, struct {
		chatID int64
		text   string
	}{chatID, text})
	if f.err != nil {
		return 0, f.err
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

func (f *fakePubStore) GetByID(ctx context.Context, id int64) (*Publication, error) {
	p, ok := f.pubs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakePubStore) List(ctx context.Context) ([]Publication, error) {
	out := make([]Publication, 0, len(f.pubs))
	for _, p := range f.pubs {
		out = append(out, *p)
	}
	return out, nil
}

func (f *fakePubStore) UpdateStatus(ctx context.Context, id int64, status Status, messageID *int64, errMsg *string) error {
	p, ok := f.pubs[id]
	if !ok {
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

func TestService_PublishSuccess(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 123}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	pub, err := svc.Publish(context.Background(), 7, -1001, "Hola mundo")
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
	// Log de exito.
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

	_, err := svc.Publish(context.Background(), 7, -1001, "")
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

	longText := make([]byte, 4097)
	for i := range longText {
		longText[i] = 'a'
	}
	_, err := svc.Publish(context.Background(), 7, -1001, string(longText))
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

	_, err := svc.Publish(context.Background(), 7, -999, "Hola")
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

	_, err := svc.Publish(context.Background(), 7, -1001, "Hola")
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
	if e.ErrorMessage == nil || *e.ErrorMessage == "" {
		t.Error("error_message vacio en log de permiso")
	}
}

func TestService_PublishTelegramError(t *testing.T) {
	tg := &fakeTelegramPub{err: telegram.ErrPermissionDenied}
	store := newFakePubStore()
	svc, logFake := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	_, err := svc.Publish(context.Background(), 7, -1001, "Hola")
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Fatalf("Publish() error = %v, want ErrPermissionDenied del adapter", err)
	}
	// La publicacion queda con status failed y error_message.
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
		t.Errorf("log status = %s, want PERMISSION_DENIED (mapeo del error de Telegram)", logFake.entries[0].Status)
	}
	if logFake.entries[0].ErrorMessage == nil {
		t.Error("error_message ausente en log")
	}
}

func TestService_List(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})

	// Crea dos publicaciones.
	if _, err := svc.Publish(context.Background(), 7, -1001, "Primera"); err != nil {
		t.Fatalf("Publish 1 error: %v", err)
	}
	if _, err := svc.Publish(context.Background(), 7, -1001, "Segunda"); err != nil {
		t.Fatalf("Publish 2 error: %v", err)
	}

	list, err := svc.List(context.Background())
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

	pub, err := svc.Publish(context.Background(), 7, -1001, "Hola")
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	// Encontrado.
	got, err := svc.GetByID(context.Background(), pub.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.ID != pub.ID || got.Text != "Hola" {
		t.Errorf("got = %+v, want publicacion con texto 'Hola'", got)
	}

	// No encontrado.
	_, err = svc.GetByID(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(999) error = %v, want ErrNotFound", err)
	}
}
