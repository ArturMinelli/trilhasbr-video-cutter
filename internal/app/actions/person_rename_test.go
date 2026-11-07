package actions

import (
	"context"
	"strings"
	"testing"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/repository"
)

// labelRepo records SetGroupLabel calls; other methods panic on the nil embed.
type labelRepo struct {
	repository.DetectionRepository
	calls []string
}

func (r *labelRepo) SetGroupLabel(_ context.Context, _ int64, label string) error {
	r.calls = append(r.calls, label)
	return nil
}

func TestPersonListRename(t *testing.T) {
	cases := []struct {
		name    string
		groupID int64
		label   string
		want    string // stored label when accepted
		invalid bool
	}{
		{name: "plain", groupID: 1, label: "Ana", want: "Ana"},
		{name: "trimmed", groupID: 1, label: "  Ana Maria \t ", want: "Ana Maria"},
		{name: "whitespace only clears", groupID: 1, label: "   ", want: ""},
		{name: "empty clears", groupID: 1, label: "", want: ""},
		{name: "60 ascii ok", groupID: 1, label: strings.Repeat("a", 60), want: strings.Repeat("a", 60)},
		{name: "61 ascii rejected", groupID: 1, label: strings.Repeat("a", 61), invalid: true},
		{name: "60 multibyte runes ok (120 bytes)", groupID: 1, label: strings.Repeat("é", 60), want: strings.Repeat("é", 60)},
		{name: "61 multibyte runes rejected", groupID: 1, label: strings.Repeat("é", 61), invalid: true},
		{name: "newline rejected", groupID: 1, label: "a\nb", invalid: true},
		{name: "tab inside rejected", groupID: 1, label: "a\tb", invalid: true},
		{name: "NUL rejected", groupID: 1, label: "a\x00b", invalid: true},
		{name: "bad group id", groupID: 0, label: "Ana", invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &labelRepo{}
			err := (&PersonListAction{Repo: repo}).Rename(context.Background(), tc.groupID, tc.label)
			if tc.invalid {
				if appErr := apperrors.As(err); appErr == nil || appErr.Code != apperrors.CodeValidation {
					t.Fatalf("want VALIDATION_ERROR, got %v", err)
				}
				if len(repo.calls) != 0 {
					t.Fatalf("repo must not be called on invalid input: %v", repo.calls)
				}
				return
			}
			if err != nil || len(repo.calls) != 1 || repo.calls[0] != tc.want {
				t.Fatalf("err=%v calls=%q want %q", err, repo.calls, tc.want)
			}
		})
	}
}
