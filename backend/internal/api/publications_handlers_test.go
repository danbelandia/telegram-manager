package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/telegram-manager/backend/internal/publications"
)

// fakePublicationStore satisface publicationStore para los handlers.
type fakePublicationStore struct {
	pub   *publications.Publication
	list  []publications.Publication
	err   error
	calls int
	actor int64
	group int64
	text  string
}

func (f *fakePublicationStore) Publish(ctx context.Context, actorID, groupID int64, text string) (*publications.Publication, error) {
	f.calls++
	f.actor = actorID
	f.group = groupID
	f.text = text
	if f.err != nil {
		return nil, f.err
	}
	return f.pub, nil
}

func (f *fakePublicationStore) GetByID(ctx context.Context, id int64) (*publications.Publication, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.pub == nil {
		return nil, publications.ErrNotFound
	}
	return f.pub, nil
}

func (f *fakePublicationStore) List(ctx context.Context) ([]publications.Publication, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

// buildPublicationsServer construye un Server con auth fija y el modulo
// de publicaciones habilitado (fake inyectado).
func buildPublicationsServer(t *testing.T, pub *fakePublicationStore, opts ...Option) *Server {
	t.Helper()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithPublications(pub),
	)
	for _, o := range opts {
		o(server)
	}
	return server
}

func TestPublicationsRoutes_RequireAuth(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	cases := []struct {
		method, path string
	}{
		{"POST", "/api/publications"},
		{"GET", "/api/publications"},
		{"GET", "/api/publications/1"},
	}
	for _, tc := range cases {
		rr := doRequest(server, tc.method, tc.path, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: code = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

func TestPublications_CreateSuccess(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{pub: &publications.Publication{
		ID:         1,
		TelegramID: -100123,
		Text:       "Hola mundo",
		Status:     publications.StatusSent,
		MessageID:  &mid,
		ActorID:    int64Ptr(1),
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications", `{"text":"Hola mundo","group_id":-100123}`, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if pub.calls != 1 || pub.actor != 1 || pub.group != -100123 || pub.text != "Hola mundo" {
		t.Errorf("Publish params = actor=%d group=%d text=%s, want actor=1 group=-100123 text='Hola mundo'", pub.actor, pub.group, pub.text)
	}
	// Envelope con data poblada.
	if !strings.Contains(rr.Body.String(), `"status":"sent"`) {
		t.Errorf("body sin status sent: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"error":null`) {
		t.Errorf("body sin error null: %s", rr.Body.String())
	}
}

func TestPublications_CreateEmptyText(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrTextEmpty}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications", `{"text":"","group_id":-100123}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
}

func TestPublications_CreateGroupNotFound(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrGroupNotFound}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications", `{"text":"Hola","group_id":-999}`, validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "NOT_FOUND") {
		t.Errorf("body sin NOT_FOUND: %s", rr.Body.String())
	}
}

func TestPublications_CreatePermissionDenied(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrBotPermission}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications", `{"text":"Hola","group_id":-100123}`, validToken)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "PERMISSION_DENIED") {
		t.Errorf("body sin PERMISSION_DENIED: %s", rr.Body.String())
	}
}

func TestPublications_GetByID(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{pub: &publications.Publication{
		ID:         1,
		TelegramID: -100123,
		Text:       "Hola mundo",
		Status:     publications.StatusSent,
		MessageID:  &mid,
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications/1", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"id":1`) {
		t.Errorf("body sin id 1: %s", rr.Body.String())
	}
}

func TestPublications_GetByIDNotFound(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications/999", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestPublications_List(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{list: []publications.Publication{
		{ID: 1, TelegramID: -100123, Text: "Primera", Status: publications.StatusSent, MessageID: &mid},
		{ID: 2, TelegramID: -100123, Text: "Segunda", Status: publications.StatusFailed},
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"text":"Primera"`) || !strings.Contains(rr.Body.String(), `"text":"Segunda"`) {
		t.Errorf("body sin ambas publicaciones: %s", rr.Body.String())
	}
}
