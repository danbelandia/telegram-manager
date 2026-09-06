package events

import (
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/telegram"
)

func TestBusPublish_SingleHandler(t *testing.T) {
	bus := NewBus()
	got := make(chan *telegram.Update, 1)
	bus.Handle(func(u *telegram.Update) { got <- u })

	update := &telegram.Update{UpdateID: 10, Message: &telegram.Message{Text: "hola"}}
	bus.Publish(update)

	select {
	case received := <-got:
		if received.UpdateID != 10 {
			t.Errorf("UpdateID = %d, want 10", received.UpdateID)
		}
	case <-time.After(time.Second):
		t.Fatal("handler never called")
	}
}

func TestBusPublish_MultipleHandlersCalledOnce(t *testing.T) {
	bus := NewBus()
	first := make(chan int, 1)
	second := make(chan int, 1)

	bus.Handle(func(u *telegram.Update) { first <- int(u.UpdateID) })
	bus.Handle(func(u *telegram.Update) { second <- int(u.UpdateID) })

	bus.Publish(&telegram.Update{UpdateID: 5})

	for name, ch := range map[string]chan int{"first": first, "second": second} {
		select {
		case id := <-ch:
			if id != 5 {
				t.Errorf("%s handler got %d, want 5", name, id)
			}
		case <-time.After(time.Second):
			t.Errorf("%s handler never called", name)
		}
	}

	// Publish no debe duplicar entregas.
	select {
	case <-first:
		t.Error("first handler called twice")
	case <-second:
		t.Error("second handler called twice")
	case <-time.After(100 * time.Millisecond):
		// OK: no hay duplicados.
	}
}
