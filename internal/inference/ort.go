package inference

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"trilhasbr-video-cutter/internal/apperrors"
)

// Provider selects the execution provider. Auto = CoreML for YOLO (CPU fallback), CPU for OSNet.
type Provider string

const (
	ProviderAuto   Provider = "auto"
	ProviderCPU    Provider = "cpu"
	ProviderCoreML Provider = "coreml"
)

type Config struct {
	LibPath   string // libonnxruntime path
	YOLOPath  string
	OSNetPath string
	Sessions  int      // parallel session pairs (default 2)
	Provider  Provider // default auto
	Logger    *slog.Logger
}

type yoloSession struct {
	s   *ort.DynamicAdvancedSession
	in  *ort.Tensor[float32]
	out *ort.Tensor[float32]
}

type osnetSession struct {
	s *ort.DynamicAdvancedSession
}

// ORT is the ONNX Runtime backed Engine.
type ORT struct {
	yolo     chan *yoloSession
	osnet    chan *osnetSession
	yoloAll  []*yoloSession
	osnetAll []*osnetSession
	// Provider reports what was actually used ("coreml" or "cpu") per model.
	YOLOProvider, OSNetProvider string
}

var (
	envOnce sync.Once
	envErr  error
)

// Open loads the runtime and models. ponytail: ort environment is process-global, so Open
// must use the same LibPath for the whole process lifetime.
func Open(cfg Config) (*ORT, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Sessions <= 0 {
		cfg.Sessions = 2
	}
	if cfg.Provider == "" {
		cfg.Provider = ProviderAuto
	}
	envOnce.Do(func() {
		ort.SetSharedLibraryPath(cfg.LibPath)
		envErr = ort.InitializeEnvironment()
	})
	if envErr != nil {
		return nil, apperrors.Wrap(CodeInference, "failed to initialize onnxruntime", envErr, map[string]any{"lib": cfg.LibPath})
	}

	yIn, yOut, err := ort.GetInputOutputInfo(cfg.YOLOPath)
	if err != nil || len(yIn) != 1 || len(yOut) != 1 {
		return nil, inferenceErr("failed to inspect yolo model", err)
	}
	oIn, oOut, err := ort.GetInputOutputInfo(cfg.OSNetPath)
	if err != nil || len(oIn) != 1 || len(oOut) != 1 {
		return nil, inferenceErr("failed to inspect osnet model", err)
	}

	e := &ORT{yolo: make(chan *yoloSession, cfg.Sessions), osnet: make(chan *osnetSession, cfg.Sessions)}
	for i := 0; i < cfg.Sessions; i++ {
		ys, prov, err := openYOLO(cfg, yIn[0].Name, yOut[0].Name)
		if err != nil {
			e.Close()
			return nil, err
		}
		os, oprov, err := openOSNet(cfg, oIn[0].Name, oOut[0].Name)
		if err != nil {
			e.Close()
			return nil, err
		}
		e.yoloAll, e.osnetAll = append(e.yoloAll, ys), append(e.osnetAll, os)
		e.yolo <- ys
		e.osnet <- os
		e.YOLOProvider, e.OSNetProvider = prov, oprov
	}
	cfg.Logger.Info("inference ready", "sessions", cfg.Sessions, "yoloProvider", e.YOLOProvider, "osnetProvider", e.OSNetProvider)
	return e, nil
}

// sessionOptions returns options and the provider name actually attached.
func sessionOptions(cfg Config) (*ort.SessionOptions, string, error) {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, "", inferenceErr("failed to create session options", err)
	}
	if cfg.Provider == ProviderCPU || runtime.GOOS != "darwin" {
		return opts, "cpu", nil
	}
	err = opts.AppendExecutionProviderCoreMLV2(map[string]string{"MLComputeUnits": "ALL", "ModelFormat": "MLProgram"})
	if err != nil {
		cfg.Logger.Warn("coreml unavailable, using cpu", "error", err)
		opts.Destroy()
		opts, err = ort.NewSessionOptions()
		if err != nil {
			return nil, "", inferenceErr("failed to create session options", err)
		}
		return opts, "cpu", nil
	}
	return opts, "coreml", nil
}

func openYOLO(cfg Config, inName, outName string) (*yoloSession, string, error) {
	in, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, yoloSize, yoloSize))
	if err != nil {
		return nil, "", inferenceErr("yolo input tensor", err)
	}
	out, err := ort.NewEmptyTensor[float32](ort.NewShape(1, yoloChannels, yoloAnchors))
	if err != nil {
		in.Destroy()
		return nil, "", inferenceErr("yolo output tensor", err)
	}
	for {
		opts, prov, err := sessionOptions(cfg)
		if err != nil {
			in.Destroy()
			out.Destroy()
			return nil, "", err
		}
		s, err := ort.NewDynamicAdvancedSession(cfg.YOLOPath, []string{inName}, []string{outName}, opts)
		opts.Destroy()
		if err == nil {
			return &yoloSession{s: s, in: in, out: out}, prov, nil
		}
		if prov == "cpu" || cfg.Provider == ProviderCoreML {
			in.Destroy()
			out.Destroy()
			return nil, "", inferenceErr("failed to load yolo model", err)
		}
		cfg.Logger.Warn("coreml session failed for yolo, retrying on cpu", "error", err)
		cfg.Provider = ProviderCPU
	}
}

func openOSNet(cfg Config, inName, outName string) (*osnetSession, string, error) {
	// ponytail: CoreML recompiles OSNet for every new batch size (~1.1 s each), while CPU is ~5-10 ms/crop,
	// so auto means CPU here. Pad batches to a fixed size if CoreML for re-ID is ever wanted.
	if cfg.Provider == ProviderAuto {
		cfg.Provider = ProviderCPU
	}
	for {
		opts, prov, err := sessionOptions(cfg)
		if err != nil {
			return nil, "", err
		}
		s, err := ort.NewDynamicAdvancedSession(cfg.OSNetPath, []string{inName}, []string{outName}, opts)
		opts.Destroy()
		if err == nil {
			return &osnetSession{s: s}, prov, nil
		}
		if prov == "cpu" || cfg.Provider == ProviderCoreML {
			return nil, "", inferenceErr("failed to load osnet model", err)
		}
		cfg.Logger.Warn("coreml session failed for osnet, retrying on cpu", "error", err)
		cfg.Provider = ProviderCPU
	}
}

func (e *ORT) Detect(f Frame) ([]Box, error) {
	if f.W <= 0 || f.H <= 0 || len(f.RGB) != f.W*f.H*3 {
		return nil, apperrors.New(apperrors.CodeValidation, "invalid frame", map[string]any{"w": f.W, "h": f.H, "len": len(f.RGB)})
	}
	s := <-e.yolo
	defer func() { e.yolo <- s }()

	scale, padX, padY := letterbox(f, s.in.GetData())
	if err := s.s.Run([]ort.Value{s.in}, []ort.Value{s.out}); err != nil {
		return nil, inferenceErr("yolo inference failed", err)
	}
	return decodeYOLO(s.out.GetData(), scale, padX, padY, f.W, f.H), nil
}

func (e *ORT) Embed(crops []Frame) ([][]float32, error) {
	s := <-e.osnet
	defer func() { e.osnet <- s }()

	res := make([][]float32, 0, len(crops))
	for start := 0; start < len(crops); start += osnetMaxBatch {
		batch := crops[start:min(start+osnetMaxBatch, len(crops))]
		in, err := ort.NewTensor(ort.NewShape(int64(len(batch)), 3, osnetH, osnetW), osnetInput(batch))
		if err != nil {
			return nil, inferenceErr("osnet input tensor", err)
		}
		outs := []ort.Value{nil}
		err = s.s.Run([]ort.Value{in}, outs)
		in.Destroy()
		if err != nil {
			return nil, inferenceErr("osnet inference failed", err)
		}
		flat := outs[0].(*ort.Tensor[float32]).GetData()
		for i := range batch {
			v := make([]float32, EmbeddingDim)
			copy(v, flat[i*EmbeddingDim:(i+1)*EmbeddingDim])
			l2(v)
			res = append(res, v)
		}
		outs[0].Destroy()
	}
	return res, nil
}

func (e *ORT) Close() error {
	for _, s := range e.yoloAll {
		s.s.Destroy()
		s.in.Destroy()
		s.out.Destroy()
	}
	for _, s := range e.osnetAll {
		s.s.Destroy()
	}
	e.yoloAll, e.osnetAll = nil, nil
	return nil
}

func inferenceErr(msg string, err error) error {
	if err == nil {
		err = fmt.Errorf("unexpected model signature")
	}
	return apperrors.Wrap(CodeInference, msg, err, nil)
}

var _ Engine = (*ORT)(nil)
