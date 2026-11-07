package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	wailsapi "trilhasbr-video-cutter/internal/api/wails"
	"trilhasbr-video-cutter/internal/app/actions"
	"trilhasbr-video-cutter/internal/app/jobs"
	"trilhasbr-video-cutter/internal/inference"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository/sqlite"
)

type Dependencies struct {
	Service      *wailsapi.Service
	DB           *sql.DB
	MediaHandler http.Handler
	// Close releases the inference engine (if it was opened) and the database.
	Close func()
}

func Build(ctx context.Context) (*Dependencies, error) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	appDataDir, err := platform.AppDataDir("trilhasbr-video-cutter")
	if err != nil {
		return nil, err
	}
	databasePath := sqlite.MustDBPath(appDataDir)

	db, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		return nil, err
	}

	if err = sqlite.RunMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	workspaceRepo := sqlite.NewWorkspaceRepository(db)
	videoRepo := sqlite.NewVideoRepository(db)

	bundledFFmpegRoot := filepath.Join("resources", "ffmpeg")
	resolver := platform.NewFFmpegResolver(bundledFFmpegRoot)
	media := ffmpeg.NewClient(resolver, logger)

	bridge := wailsapi.NewRuntimeBridge()

	detectJobs := jobs.NewDetectionManager()
	busyGuard := actions.WorkspaceBusyGuard(detectJobs.WorkspaceBusy)

	workspaceListAction := &actions.WorkspaceListAction{WorkspaceRepo: workspaceRepo}
	workspaceLoadAction := &actions.WorkspaceLoadAction{
		Picker:        bridge,
		WorkspaceRepo: workspaceRepo,
		VideoRepo:     videoRepo,
		MediaProbe:    media,
		Logger:        logger,
		Guard:         busyGuard,
	}
	workspaceDeleteAction := &actions.WorkspaceDeleteAction{
		WorkspaceRepo: workspaceRepo,
		// Per-workspace app artifacts: preview thumbnails and people crops (same roots the actions write to).
		ArtifactRoots: []string{filepath.Join(appDataDir, "thumbnails"), filepath.Join(appDataDir, "people")},
		Guard:         busyGuard,
		Logger:        logger,
	}
	workspaceExplorerAction := &actions.WorkspaceExplorerAction{
		WorkspaceRepo: workspaceRepo,
		VideoRepo:     videoRepo,
	}
	videoListAction := &actions.VideoListAction{VideoRepo: videoRepo}
	videoPreviewAction := &actions.VideoPreviewAction{
		VideoRepo:      videoRepo,
		MediaProbe:     media,
		MediaThumbnail: media,
		ThumbnailRoot:  filepath.Join(appDataDir, "thumbnails"),
		PreviewWidth:   320,
	}
	videoShiftAction := &actions.VideoShiftAction{
		WorkspaceRepo: workspaceRepo,
		VideoRepo:     videoRepo,
	}
	cutExecuteAction := &actions.CutExecuteAction{
		WorkspaceRepo: workspaceRepo,
		VideoRepo:     videoRepo,
		Media:         media,
		Logger:        logger,
	}
	cutCSVPreviewAction := &actions.CutCSVPreviewAction{
		Picker:    bridge,
		Workspace: workspaceRepo,
		Planner:   cutExecuteAction,
	}
	cutCSVExecuteAction := &actions.CutCSVExecuteAction{
		Workspace: workspaceRepo,
		Runner:    cutExecuteAction,
	}
	jobManager := jobs.NewManager()

	detectionRepo := sqlite.NewDetectionRepository(db)
	personListAction := &actions.PersonListAction{Repo: detectionRepo}
	personCutAction := &actions.PersonCutAction{
		WorkspaceRepo: workspaceRepo,
		VideoRepo:     videoRepo,
		Detection:     detectionRepo,
		Runner:        cutExecuteAction,
	}

	// The ONNX engine is opened on the first detection run so the app still starts
	// (and cuts still work) when models or the runtime library are missing.
	var (
		engineMu sync.Mutex
		engine   *inference.ORT
		version  string
	)
	detectRun := func() (*actions.DetectRunAction, error) {
		engineMu.Lock()
		defer engineMu.Unlock()
		if engine == nil {
			paths, resolveErr := platform.NewModelResolver("resources").Resolve()
			if resolveErr != nil {
				return nil, resolveErr
			}
			opened, openErr := inference.Open(inference.Config{
				LibPath: paths.ORTLib, YOLOPath: paths.YOLO, OSNetPath: paths.OSNet, Logger: logger,
			})
			if openErr != nil {
				return nil, openErr
			}
			// The "+path1" suffix is the pipeline-version knob: analyses stored before per-frame box paths
			// existed have a different model_version, so each video is re-analyzed once. Bump it again
			// whenever the stored tracking output changes shape.
			engine, version = opened, paths.Version+"+path1"
		}
		return &actions.DetectRunAction{
			WorkspaceRepo: workspaceRepo,
			VideoRepo:     videoRepo,
			DetectionRepo: detectionRepo,
			Media:         media,
			Engine:        engine,
			ModelVersion:  version,
			ThumbDir:      filepath.Join(appDataDir, "people"),
			Logger:        logger,
		}, nil
	}

	service := wailsapi.NewService(wailsapi.ServiceParams{
		Logger:          logger,
		Bridge:          bridge,
		WorkspaceList:   workspaceListAction,
		WorkspaceLoad:   workspaceLoadAction,
		WorkspaceDelete: workspaceDeleteAction,
		WorkspaceExpo:   workspaceExplorerAction,
		VideoList:       videoListAction,
		VideoPreview:    videoPreviewAction,
		VideoShift:      videoShiftAction,
		CutExecute:      cutExecuteAction,
		CutCSVPreview:   cutCSVPreviewAction,
		CutCSVExecute:   cutCSVExecuteAction,
		JobManager:      jobManager,
		DetectRun:       detectRun,
		DetectJobs:      detectJobs,
		PersonList:      personListAction,
		PersonCut:       personCutAction,
	})

	return &Dependencies{
		Service:      service,
		DB:           db,
		MediaHandler: &wailsapi.MediaHandler{Videos: videoRepo},
		Close: func() {
			engineMu.Lock()
			if engine != nil {
				_ = engine.Close()
			}
			engineMu.Unlock()
			_ = db.Close()
		},
	}, nil
}
