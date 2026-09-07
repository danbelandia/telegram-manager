// Tests del modelo publications (slice 3: scheduled_at + paginacion).
// Los helpers de validacion son puros (sin I/O) y se inyecta `nowFn`
// para determinismo.
package publications

import (
	"errors"
	"testing"
	"time"
)

// fixedNow devuelve siempre el mismo instante (helper local).
func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestNormalizeScheduledAt_FutureUTC_OK(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	future := now.Add(2 * time.Hour)

	got, err := NormalizeScheduledAt(future.Format(time.RFC3339), fixedNow(now))
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("returned nil, want non-nil")
	}
	if !got.Equal(future) {
		t.Errorf("parsed = %v, want %v", got, future)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
}

func TestNormalizeScheduledAt_Past_ReturnsErrScheduledInPast(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Minute)

	_, err := NormalizeScheduledAt(past.Format(time.RFC3339), fixedNow(now))
	if !errors.Is(err, ErrScheduledInPast) {
		t.Fatalf("error = %v, want ErrScheduledInPast", err)
	}
}

func TestNormalizeScheduledAt_Offset_NormalizesToUTC(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	// 2027-01-01 17:00:00 +03:00 == 14:00:00 UTC (futuro lejano).
	withOffset := "2027-01-01T17:00:00+03:00"

	got, err := NormalizeScheduledAt(withOffset, fixedNow(now))
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC (normalizado)", got.Location())
	}
	expected := time.Date(2027, 1, 1, 14, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("normalized = %v, want %v", got, expected)
	}
}

func TestNormalizeScheduledAt_InvalidString_ReturnsErrScheduledInPast(t *testing.T) {
	_, err := NormalizeScheduledAt("no-es-rfc3339", nil)
	if !errors.Is(err, ErrScheduledInPast) {
		t.Fatalf("error = %v, want ErrScheduledInPast", err)
	}
}

func TestValidateScheduledAt_FutureOK(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	if err := validateScheduledAt(now.Add(time.Second), fixedNow(now)); err != nil {
		t.Errorf("error = %v, want nil para futuro estricto", err)
	}
}

func TestValidateScheduledAt_EqualNow_ReturnsErr(t *testing.T) {
	// Tolerancia cero: igual a now() es invalido.
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	if err := validateScheduledAt(now, fixedNow(now)); !errors.Is(err, ErrScheduledInPast) {
		t.Errorf("error = %v, want ErrScheduledInPast (igual a now)", err)
	}
}

func TestValidateScheduledAt_Past_ReturnsErr(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	if err := validateScheduledAt(now.Add(-time.Second), fixedNow(now)); !errors.Is(err, ErrScheduledInPast) {
		t.Errorf("error = %v, want ErrScheduledInPast", err)
	}
}

func TestValidatePagination_OK(t *testing.T) {
	cases := []struct{ limit, offset int }{
		{1, 0}, {50, 0}, {100, 0}, {50, 10}, {100, 999},
	}
	for _, c := range cases {
		if err := ValidatePagination(c.limit, c.offset); err != nil {
			t.Errorf("ValidatePagination(%d,%d) = %v, want nil", c.limit, c.offset, err)
		}
	}
}

func TestValidatePagination_OutOfRange(t *testing.T) {
	cases := []struct {
		name          string
		limit, offset int
	}{
		{"limit=0", 0, 0},
		{"limit=101", 101, 0},
		{"limit=-1", -1, 0},
		{"offset=-1", 50, -1},
	}
	for _, c := range cases {
		if err := ValidatePagination(c.limit, c.offset); !errors.Is(err, ErrInvalidPagination) {
			t.Errorf("%s: error = %v, want ErrInvalidPagination", c.name, err)
		}
	}
}

func TestNormalizePagination_Default(t *testing.T) {
	// Ambos cero -> default 50, 0.
	limit, offset := NormalizePagination(0, 0)
	if limit != defaultListLimit || offset != 0 {
		t.Errorf("default = (%d,%d), want (%d,0)", limit, offset, defaultListLimit)
	}
}

func TestNormalizePagination_Passthrough(t *testing.T) {
	limit, offset := NormalizePagination(25, 100)
	if limit != 25 || offset != 100 {
		t.Errorf("passthrough = (%d,%d), want (25,100)", limit, offset)
	}
}
