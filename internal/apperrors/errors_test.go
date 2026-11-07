package apperrors

import (
	"errors"
	"testing"
)

func TestToPublic_UsesExistingCode(t *testing.T) {
	err := Wrap(CodeDB, "db failed", errors.New("boom"), map[string]any{"k": "v"})
	public := ToPublic(err)
	if public == nil {
		t.Fatal("expected public error")
	}
	if public.Code != CodeDB {
		t.Fatalf("expected code %s, got %s", CodeDB, public.Code)
	}
	if public.Message != "db failed" {
		t.Fatalf("unexpected message %s", public.Message)
	}
}

func TestToPublic_MapsGenericErrorToInternal(t *testing.T) {
	public := ToPublic(errors.New("x"))
	if public == nil {
		t.Fatal("expected public error")
	}
	if public.Code != CodeInternal {
		t.Fatalf("expected code %s, got %s", CodeInternal, public.Code)
	}
}
