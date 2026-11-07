// Package tracking is a ByteTrack-style multi-object tracker (pure Go) with an optional
// appearance (re-ID embedding) term, tuned for sparsely sampled video frames.
package tracking

import (
	"math"
	"sort"
)

// Box is x1, y1, x2, y2 in pixels.
type Box [4]float64

// Detection is one person detection in one sampled frame.
type Detection struct {
	Box        Box
	Score      float64
	FrameIndex int       // index of the sampled frame (copied into Track.Best)
	Embedding  []float32 // optional, L2-normalized re-ID vector; nil if not computed
}

// BestDetection is the highest-scoring detection of a track (used for the thumbnail).
type BestDetection struct {
	Box        Box
	FrameIndex int
	TimeSec    float64
	Score      float64
}

// PathPoint is one matched detection of a track (pixel coordinates as given to Update).
type PathPoint struct {
	TimeSec float64
	Box     Box
}

// Track is a finalized per-video track. Only confirmed tracks (>= MinHits) are returned.
type Track struct {
	ID         int // 1-based, assigned when the track is confirmed
	StartSec   float64
	EndSec     float64
	Detections int
	MeanScore  float64
	Best       BestDetection
	Path       []PathPoint // every matched detection, in time order
	Embedding  []float32   // mean of detection embeddings, L2-normalized; nil if none
}

// Config holds tuning constants. Zero values take the documented defaults.
type Config struct {
	FPS            float64 // sample rate in frames per second (default 5)
	HighScore      float64 // first-stage detections: score >= this (default 0.5)
	LowScore       float64 // second-stage detections: LowScore <= score < HighScore (default 0.35)
	MatchThresh    float64 // max blended cost in stage 1 (default 0.8)
	LowMatchThresh float64 // max (1-IoU) in stage 2 (default 0.5)
	EmbWeight      float64 // weight of cosine distance in the blended cost (default 0.3)
	EmbAlpha       float64 // EMA factor for the track appearance feature (default 0.9)
	LostSeconds    float64 // video time a track survives without a match (default 2)
	MinHits        int     // matches needed to confirm a track (default 3)
}

func (c Config) withDefaults() Config {
	def := func(v *float64, d float64) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&c.FPS, 5)
	def(&c.HighScore, 0.5)
	def(&c.LowScore, 0.35)
	def(&c.MatchThresh, 0.8)
	def(&c.LowMatchThresh, 0.5)
	def(&c.EmbWeight, 0.3)
	def(&c.EmbAlpha, 0.9)
	def(&c.LostSeconds, 2)
	if c.MinHits <= 0 {
		c.MinHits = 3
	}
	return c
}

// Tracker links detections over time. Not safe for concurrent use; use one per video.
type Tracker struct {
	cfg      Config
	tracks   []*trk
	finished []Track
	lastT    float64
	started  bool
	nextID   int
}

func New(cfg Config) *Tracker { return &Tracker{cfg: cfg.withDefaults()} }

// Update feeds the detections of one sampled frame at video time t (seconds).
// Call it for every sampled frame, including frames without detections.
func (tr *Tracker) Update(t float64, dets []Detection) {
	dtFrames := 0.0
	if tr.started && t > tr.lastT {
		dtFrames = (t - tr.lastT) * tr.cfg.FPS
	}
	tr.started, tr.lastT = true, t
	for _, k := range tr.tracks {
		k.predict(dtFrames)
	}

	var high, low []int
	for i, d := range dets {
		switch {
		case d.Score >= tr.cfg.HighScore:
			high = append(high, i)
		case d.Score >= tr.cfg.LowScore:
			low = append(low, i)
		}
	}

	matched := make(map[*trk]bool)

	// Stage 1: all tracks vs high-score detections, IoU blended with appearance.
	cost := make([][]float64, len(tr.tracks))
	for i, k := range tr.tracks {
		cost[i] = make([]float64, len(high))
		for j, di := range high {
			cost[i][j] = tr.pairCost(k, &dets[di])
		}
	}
	pairs, rowsLeft, colsLeft := assign(cost, len(high), tr.cfg.MatchThresh)
	for _, p := range pairs {
		k := tr.tracks[p[0]]
		k.observe(t, &dets[high[p[1]]], tr.cfg, &tr.nextID)
		matched[k] = true
	}

	// Stage 2: still-unmatched tracks that were matched last frame vs low-score detections (IoU only).
	var cand []*trk
	for _, i := range rowsLeft {
		if tr.tracks[i].matchedLast {
			cand = append(cand, tr.tracks[i])
		}
	}
	cost = make([][]float64, len(cand))
	for i, k := range cand {
		cost[i] = make([]float64, len(low))
		for j, di := range low {
			cost[i][j] = 1 - iou(k.box(), dets[di].Box)
		}
	}
	pairs, _, _ = assign(cost, len(low), tr.cfg.LowMatchThresh)
	for _, p := range pairs {
		k := cand[p[0]]
		k.observe(t, &dets[low[p[1]]], tr.cfg, &tr.nextID)
		matched[k] = true
	}

	// Housekeeping: drop unmatched tentative tracks, expire lost ones, finalize.
	keep := tr.tracks[:0]
	for _, k := range tr.tracks {
		k.matchedLast = matched[k]
		switch {
		case matched[k]:
			keep = append(keep, k)
		case k.id == 0: // never confirmed
		case t-k.endT > tr.cfg.LostSeconds+0.5/tr.cfg.FPS:
			tr.finished = append(tr.finished, k.finalize())
		default:
			keep = append(keep, k)
		}
	}
	tr.tracks = keep

	// New tracks from unmatched high-score detections.
	for _, j := range colsLeft {
		d := &dets[high[j]]
		k := newTrk(d.Box)
		k.observe(t, d, tr.cfg, &tr.nextID)
		k.matchedLast = true
		tr.tracks = append(tr.tracks, k)
	}
}

// Finish finalizes every remaining track and returns all confirmed tracks ordered by start time.
func (tr *Tracker) Finish() []Track {
	for _, k := range tr.tracks {
		if k.id != 0 {
			tr.finished = append(tr.finished, k.finalize())
		}
	}
	tr.tracks = nil
	out := tr.finished
	tr.finished = nil
	sort.Slice(out, func(a, b int) bool {
		if out[a].StartSec != out[b].StartSec {
			return out[a].StartSec < out[b].StartSec
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// pairCost blends 1-IoU with cosine distance when both sides have an embedding.
// A zero-IoU pair is only feasible for a lost track (re-identification after occlusion).
func (tr *Tracker) pairCost(k *trk, d *Detection) float64 {
	ov := iou(k.box(), d.Box)
	if ov == 0 && k.matchedLast {
		return 1
	}
	c := 1 - ov
	if len(k.ema) > 0 && len(k.ema) == len(d.Embedding) {
		e := math.Min(1, math.Max(0, 1-dot(k.ema, d.Embedding)))
		c = (1-tr.cfg.EmbWeight)*c + tr.cfg.EmbWeight*e
	}
	return c
}

// ---- per-track state ----

const (
	wPos = 1.0 / 20  // ByteTrack Kalman position noise (relative to box height)
	wVel = 1.0 / 160 // velocity noise
)

// axis is an independent constant-velocity Kalman filter for one box coordinate
// (velocity in units per sampled frame).
type axis struct{ p, v, pp, pv, vv float64 }

func (a *axis) predict(dt, h float64) {
	a.p += a.v * dt
	a.pp += dt*(2*a.pv+dt*a.vv) + sq(wPos*h)
	a.pv += dt * a.vv
	a.vv += sq(wVel * h)
}

func (a *axis) update(z, h float64) {
	k0, k1 := a.pp/(a.pp+sq(wPos*h)), a.pv/(a.pp+sq(wPos*h))
	y := z - a.p
	a.p += k0 * y
	a.v += k1 * y
	a.vv -= k1 * a.pv
	a.pp, a.pv = (1-k0)*a.pp, (1-k0)*a.pv
}

type trk struct {
	s           [4]axis // cx, cy, aspect (w/h), h
	id          int
	hits        int
	matchedLast bool
	startT      float64
	endT        float64
	sumScore    float64
	best        BestDetection
	path        []PathPoint
	ema         []float32
	embSum      []float64
}

func toState(b Box) [4]float64 {
	w, h := b[2]-b[0], math.Max(b[3]-b[1], 1)
	return [4]float64{(b[0] + b[2]) / 2, (b[1] + b[3]) / 2, w / h, h}
}

func newTrk(b Box) *trk {
	z := toState(b)
	k := &trk{}
	for i := range k.s {
		k.s[i] = axis{p: z[i], pp: sq(2 * wPos * z[3]), vv: sq(10 * wVel * z[3])}
	}
	return k
}

func (k *trk) h() float64 { return math.Max(k.s[3].p, 1) }

func (k *trk) predict(dt float64) {
	h := k.h()
	for i := range k.s {
		k.s[i].predict(dt, h)
	}
}

func (k *trk) box() Box {
	h := k.h()
	w := k.s[2].p * h
	return Box{k.s[0].p - w/2, k.s[1].p - h/2, k.s[0].p + w/2, k.s[1].p + h/2}
}

func (k *trk) observe(t float64, d *Detection, cfg Config, nextID *int) {
	if k.hits > 0 { // first observation is the init state; don't filter against itself
		z, h := toState(d.Box), k.h()
		for i := range k.s {
			k.s[i].update(z[i], h)
		}
	} else {
		k.startT = t
	}
	k.hits++
	k.endT = t
	k.sumScore += d.Score
	k.path = append(k.path, PathPoint{TimeSec: t, Box: d.Box})
	if d.Score > k.best.Score {
		k.best = BestDetection{Box: d.Box, FrameIndex: d.FrameIndex, TimeSec: t, Score: d.Score}
	}
	if e := d.Embedding; len(e) > 0 && (len(k.embSum) == 0 || len(k.embSum) == len(e)) {
		if k.embSum == nil {
			k.embSum = make([]float64, len(e))
			k.ema = append([]float32(nil), e...)
		} else {
			for i, v := range e {
				k.ema[i] = float32(cfg.EmbAlpha)*k.ema[i] + float32(1-cfg.EmbAlpha)*v
			}
			normalize(k.ema)
		}
		for i, v := range e {
			k.embSum[i] += float64(v)
		}
	}
	if k.id == 0 && k.hits >= cfg.MinHits {
		*nextID++
		k.id = *nextID
	}
}

func (k *trk) finalize() Track {
	tk := Track{ID: k.id, StartSec: k.startT, EndSec: k.endT, Detections: k.hits,
		MeanScore: k.sumScore / float64(k.hits), Best: k.best, Path: k.path}
	if len(k.embSum) > 0 {
		tk.Embedding = make([]float32, len(k.embSum))
		for i, v := range k.embSum {
			tk.Embedding[i] = float32(v)
		}
		normalize(tk.Embedding)
	}
	return tk
}

// ---- helpers ----

func sq(x float64) float64 { return x * x }

func iou(a, b Box) float64 {
	iw := math.Min(a[2], b[2]) - math.Max(a[0], b[0])
	ih := math.Min(a[3], b[3]) - math.Max(a[1], b[1])
	if iw <= 0 || ih <= 0 {
		return 0
	}
	inter := iw * ih
	return inter / ((a[2]-a[0])*(a[3]-a[1]) + (b[2]-b[0])*(b[3]-b[1]) - inter)
}

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

func normalize(v []float32) {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n = math.Sqrt(n); n > 0 {
		for i := range v {
			v[i] = float32(float64(v[i]) / n)
		}
	}
}
