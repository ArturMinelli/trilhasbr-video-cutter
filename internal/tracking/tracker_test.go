package tracking

import (
	"math"
	"testing"
)

func bx(cx, cy float64) Box { return Box{cx - 25, cy - 50, cx + 25, cy + 50} }

// frameT is the video time of sampled frame i at the default 5 fps.
func frameT(i int) float64 { return float64(i) * 0.2 }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// Identity is encoded in the score (A=0.9, B=0.6): a swap would mix the per-track means.
func findByMean(t *testing.T, tracks []Track, mean float64) Track {
	t.Helper()
	for _, tk := range tracks {
		if near(tk.MeanScore, mean) {
			return tk
		}
	}
	t.Fatalf("no track with mean score %.2f in %+v", mean, tracks)
	return Track{}
}

func TestCrossingKeepsDistinctIDs(t *testing.T) {
	tr := New(Config{})
	for i := 0; i <= 20; i++ {
		x := float64(100 + 20*i)
		tr.Update(frameT(i), []Detection{
			{Box: bx(x, x), Score: 0.9, FrameIndex: i},     // A: down-right
			{Box: bx(600-x, x), Score: 0.6, FrameIndex: i}, // B: down-left, meets A at i=10
		})
	}
	tracks := tr.Finish()
	if len(tracks) != 2 {
		t.Fatalf("want 2 tracks, got %d", len(tracks))
	}
	for _, m := range []float64{0.9, 0.6} {
		if tk := findByMean(t, tracks, m); tk.Detections != 21 {
			t.Errorf("track %.1f has %d detections, want 21", m, tk.Detections)
		}
	}
}

func TestOcclusionOneSecondKeepsTrack(t *testing.T) {
	tr := New(Config{})
	for i := 0; i < 25; i++ {
		if i >= 10 && i < 15 { // 1 s without detections
			tr.Update(frameT(i), nil)
			continue
		}
		tr.Update(frameT(i), []Detection{{Box: bx(100+10*float64(i), 300), Score: 0.8, FrameIndex: i}})
	}
	tracks := tr.Finish()
	if len(tracks) != 1 || tracks[0].Detections != 20 {
		t.Fatalf("want 1 track with 20 detections, got %+v", tracks)
	}
}

func TestAbsenceBeyondToleranceStartsNewTrack(t *testing.T) {
	tr := New(Config{})
	for i := 0; i < 35; i++ {
		if i >= 10 && i < 25 { // 3 s gap > 2 s tolerance
			tr.Update(frameT(i), nil)
			continue
		}
		tr.Update(frameT(i), []Detection{{Box: bx(300, 300), Score: 0.8, FrameIndex: i}})
	}
	tracks := tr.Finish()
	if len(tracks) != 2 || tracks[0].ID == tracks[1].ID {
		t.Fatalf("want 2 distinct tracks, got %+v", tracks)
	}
	if !near(tracks[0].EndSec, frameT(9)) || !near(tracks[1].StartSec, frameT(25)) {
		t.Errorf("unexpected times: %+v", tracks)
	}
}

// A and B sit 10px apart (heavily overlapping), then exchange places. IoU alone follows the positions (swap);
// the appearance term must keep each identity.
func TestEmbeddingResolvesIDSwap(t *testing.T) {
	run := func(withEmb bool) []Track {
		tr := New(Config{})
		eA, eB := []float32{1, 0}, []float32{0, 1}
		for i := 0; i < 12; i++ {
			xa, xb := 300.0, 310.0
			if i >= 6 {
				xa, xb = xb, xa
			}
			a := Detection{Box: bx(xa, 300), Score: 0.9, FrameIndex: i}
			b := Detection{Box: bx(xb, 300), Score: 0.6, FrameIndex: i}
			if withEmb {
				a.Embedding, b.Embedding = eA, eB
			}
			tr.Update(frameT(i), []Detection{a, b})
		}
		return tr.Finish()
	}

	with := run(true)
	if len(with) != 2 {
		t.Fatalf("with embeddings: want 2 tracks, got %d", len(with))
	}
	findByMean(t, with, 0.9)
	findByMean(t, with, 0.6)

	for _, tk := range run(false) { // negative control: without embeddings identities do swap
		if near(tk.MeanScore, 0.9) || near(tk.MeanScore, 0.6) {
			t.Fatalf("control: expected a swap without embeddings, got clean tracks")
		}
	}
}

func TestHungarianRectangular(t *testing.T) {
	cost := [][]float64{{0.1, 0.9, 0.9}, {0.9, 0.2, 0.9}, {0.9, 0.9, 0.95}, {0.3, 0.3, 0.3}}
	pairs, rows, cols := assign(cost, 3, 0.9)
	if len(pairs) != 3 || len(rows) != 1 || len(cols) != 0 {
		t.Fatalf("pairs=%v rows=%v cols=%v", pairs, rows, cols)
	}
}

func TestPathRecordsEveryMatchedDetection(t *testing.T) {
	tr := New(Config{})
	for i := 0; i < 25; i++ {
		if i >= 10 && i < 15 { // gap: no detections, so no path points either
			tr.Update(frameT(i), nil)
			continue
		}
		tr.Update(frameT(i), []Detection{{Box: bx(100+10*float64(i), 300), Score: 0.8, FrameIndex: i}})
	}
	tracks := tr.Finish()
	if len(tracks) != 1 {
		t.Fatalf("want 1 track, got %d", len(tracks))
	}
	tk := tracks[0]
	if len(tk.Path) != tk.Detections {
		t.Fatalf("path has %d points, want %d (one per matched detection)", len(tk.Path), tk.Detections)
	}
	if !near(tk.Path[0].TimeSec, tk.StartSec) || !near(tk.Path[len(tk.Path)-1].TimeSec, tk.EndSec) {
		t.Errorf("path ends %.2f..%.2f, track %.2f..%.2f", tk.Path[0].TimeSec, tk.Path[len(tk.Path)-1].TimeSec, tk.StartSec, tk.EndSec)
	}
	for i := 1; i < len(tk.Path); i++ {
		if tk.Path[i].TimeSec <= tk.Path[i-1].TimeSec {
			t.Fatalf("path not strictly increasing at %d", i)
		}
	}
	if tk.Path[0].Box != bx(100, 300) { // raw detection box, not the Kalman state
		t.Errorf("first path box %v, want the raw detection %v", tk.Path[0].Box, bx(100, 300))
	}
}
