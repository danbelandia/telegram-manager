// Package migrations expone el SQL plano de goose para que el backend
// lo embeba en el binario (go:embed). El source of truth son los
// archivos .sql; este archivo solo los hace accesibles.
package migrations

import "embed"

// FS contiene todas las migraciones SQL del directorio.
//
//go:embed *.sql
var FS embed.FS
