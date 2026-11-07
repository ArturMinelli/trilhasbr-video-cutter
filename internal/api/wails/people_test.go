package wailsapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"trilhasbr-video-cutter/internal/app/actions"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

// boxesRepo overrides only the two reads GetVideoTrackBoxes uses; other methods panic on the nil embed.
type boxesRepo struct {
	repository.DetectionRepository
	tracks   []domain.VideoTrackBoxes
	analysis *domain.VideoAnalysis
}

func (r boxesRepo) ListVideoTrackBoxes(context.Context, int64) ([]domain.VideoTrackBoxes, error) {
	return r.tracks, nil
}

func (r boxesRepo) GetVideoAnalysis(context.Context, int64) (*domain.VideoAnalysis, error) {
	return r.analysis, nil
}

func boxesService(repo boxesRepo) *Service {
	return &Service{personList: &actions.PersonListAction{Repo: repo}}
}

func TestGetVideoTrackBoxesJSONShape(t *testing.T) {
	service := boxesService(boxesRepo{
		analysis: &domain.VideoAnalysis{SampleFPS: 5},
		tracks: []domain.VideoTrackBoxes{{TrackID: 7, GroupID: 3, StartSec: 1, EndSec: 1.2,
			Boxes: []domain.TrackBox{{T: 1, X1: 0.25, Y1: 0.5, X2: 0.75, Y2: 1}, {T: 1.2, X1: 0.25, Y1: 0.5, X2: 0.75, Y2: 1}}}},
	})
	raw, err := json.Marshal(service.GetVideoTrackBoxes(42))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"videoId":42,"sampleFps":5,"tracks":[{"trackId":7,"groupId":3,"startSec":1,"endSec":1.2,` +
		`"boxes":[[1,0.25,0.5,0.75,1],[1.2,0.25,0.5,0.75,1]]}]}`
	if string(raw) != want {
		t.Fatalf("json mismatch:\n got %s\nwant %s", raw, want)
	}
}

func TestGetVideoTrackBoxesEmptyIsArrayNotNull(t *testing.T) {
	raw, err := json.Marshal(boxesService(boxesRepo{}).GetVideoTrackBoxes(9)) // never analyzed
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"videoId":9,"sampleFps":0,"tracks":[]}`; string(raw) != want {
		t.Fatalf("got %s want %s", raw, want)
	}
}

func TestGetVideoTrackBoxesRejectsBadVideoID(t *testing.T) {
	response := boxesService(boxesRepo{}).GetVideoTrackBoxes(0)
	if response.Error == nil || response.Tracks == nil || len(response.Tracks) != 0 {
		t.Fatalf("want validation error with empty tracks, got %+v", response)
	}
}

// namesRepo serves one named and one unnamed group, and records renames.
type namesRepo struct {
	repository.DetectionRepository
	renamed []string
}

func (r *namesRepo) ListGroups(context.Context, domain.GroupFilter) ([]domain.PersonGroup, int, error) {
	return []domain.PersonGroup{{ID: 1, Label: "Ana"}, {ID: 2}}, 2, nil
}

func (r *namesRepo) SetGroupLabel(_ context.Context, groupID int64, label string) error {
	r.renamed = append(r.renamed, label)
	return nil
}

func TestPersonLabelJSON(t *testing.T) {
	repo := &namesRepo{}
	service := &Service{personList: &actions.PersonListAction{Repo: repo}}

	list := service.ListPeople(ListPeopleRequest{WorkspaceID: 1, Page: 1, PageSize: 10})
	raw, err := json.Marshal(list.Items)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"id":1,"label":"Ana"`) {
		t.Fatalf("named group must carry its label: %s", raw)
	}
	if strings.Contains(string(raw), `"id":2,"label"`) {
		t.Fatalf("unnamed group must omit label: %s", raw)
	}

	boxes := boxesService(boxesRepo{tracks: []domain.VideoTrackBoxes{
		{TrackID: 1, GroupID: 1, Label: "Ana", Boxes: []domain.TrackBox{{T: 0}}},
		{TrackID: 2, GroupID: 2, Boxes: []domain.TrackBox{{T: 0}}},
	}}).GetVideoTrackBoxes(5)
	raw, _ = json.Marshal(boxes.Tracks)
	if !strings.Contains(string(raw), `"groupId":1,"label":"Ana"`) || strings.Contains(string(raw), `"groupId":2,"label"`) {
		t.Fatalf("overlay label json: %s", raw)
	}
}

func TestRenamePersonResponse(t *testing.T) {
	repo := &namesRepo{}
	service := &Service{personList: &actions.PersonListAction{Repo: repo}}

	if response := service.RenamePerson(1, "  Bia "); !response.Success || response.Error != nil || repo.renamed[0] != "Bia" {
		t.Fatalf("rename: %+v renamed=%q", response, repo.renamed)
	}
	response := service.RenamePerson(1, strings.Repeat("x", 61))
	if response.Success || response.Error == nil || response.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("too long must be VALIDATION_ERROR: %+v", response)
	}
	if len(repo.renamed) != 1 {
		t.Fatalf("invalid rename must not reach the repo: %q", repo.renamed)
	}
}
