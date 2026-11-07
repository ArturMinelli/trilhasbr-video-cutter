package wailsapi

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"trilhasbr-video-cutter/internal/repository"
)

// MediaHandler serves indexed videos at /media/<videoId> with range support (for seeking in <video>).
// Only files recorded in the database are served; the URL never carries a path.
type MediaHandler struct {
	Videos repository.VideoRepository
}

func (h *MediaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idText, ok := strings.CutPrefix(r.URL.Path, "/media/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil {
		http.NotFound(w, r)
		return
	}
	video, err := h.Videos.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(video.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}
