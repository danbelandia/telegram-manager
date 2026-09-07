// Package joinrequests modela las solicitudes de ingreso a grupos
// (AGENTS.md §10). El backend procesa los eventos chat_join_request de
// Telegram, persiste las pendientes y registra la decision de un admin
// (aprueba/rechaza). group_id y user_id son ids de Telegram; el id
// local es el requestId de las rutas (§12, design D9).
package joinrequests

import (
	"errors"
	"time"
)

// Status es el estado de una solicitud.
type Status string

// Estados posibles de una solicitud de ingreso.
const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Request es una solicitud de ingreso. DecidedAt/DecidedBy son nil
// mientras la solicitud esta pendiente.
type Request struct {
	ID          int64
	GroupID     int64
	UserID      int64
	Status      Status
	RequestedAt time.Time
	DecidedAt   *time.Time
	DecidedBy   *int64
}

// ErrNotFound se devuelve cuando el id de solicitud no existe.
var ErrNotFound = errors.New("joinrequests: not found")

// ErrNotPending se devuelve al intentar resolver una solicitud que ya
// no esta pendiente (carrera o re-llamada; el handler la detecta como
// 409 antes de llegar aca).
var ErrNotPending = errors.New("joinrequests: not pending")
