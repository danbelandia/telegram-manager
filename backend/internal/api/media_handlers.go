// Media handlers: upload y serve de archivos multimedia (fotos/videos).
// POST /api/media/upload — recibe multipart/form-valida, guarda en disco
// por tenant y devuelve la referencia para usar en publicaciones.
// GET  /api/media/{filename} — sirve el archivo desde disco.
package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MIME types permitidos y su clasificacion.
var allowedMIMETypes = map[string]string{
	"image/jpeg": "photo",
	"image/png":  "photo",
	"image/gif":  "photo",
	"image/webp": "photo",
	"video/mp4":  "video",
}

// Limites de tamaño por tipo (mismos que model.go).
const (
	maxUploadPhotoBytes = 10 * 1024 * 1024 // 10 MB
	maxUploadVideoBytes = 50 * 1024 * 1024 // 50 MB
	maxMultipartBytes   = int64(50*1024*1024 + 1024) // 50MB + margen para headers
)

// MediaStore abstracts el guardado y lectura de archivos multimedia.
type MediaStore interface {
	Save(tenantID int64, filename string, reader io.Reader) (string, error)
	ServePath(tenantID int64, filename string) (string, error)
	Open(tenantID int64, filename string) (io.ReadCloser, error)
}

// LocalMediaStore guarda archivos en disco local, organizados por tenant.
type LocalMediaStore struct {
	uploadDir string
}

// NewLocalMediaStore crea un store apuntando a uploadDir.
func NewLocalMediaStore(uploadDir string) *LocalMediaStore {
	return &LocalMediaStore{uploadDir: uploadDir}
}

// Save persiste el archivo en <uploadDir>/<tenantID>/<filename>.
func (s *LocalMediaStore) Save(tenantID int64, filename string, reader io.Reader) (string, error) {
	dir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", tenantID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("media: mkdir: %w", err)
	}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("media: create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(path) // cleanup
		return "", fmt.Errorf("media: write: %w", err)
	}
	return path, nil
}

// ServePath devuelve la ruta absoluta del archivo para http.ServeFile.
func (s *LocalMediaStore) ServePath(tenantID int64, filename string) (string, error) {
	path := filepath.Join(s.uploadDir, fmt.Sprintf("%d", tenantID), filename)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("media: not found")
	}
	return path, nil
}

// Open devuelve un ReadCloser del archivo local para re-enviarlo a Telegram.
func (s *LocalMediaStore) Open(tenantID int64, filename string) (io.ReadCloser, error) {
	path := filepath.Join(s.uploadDir, fmt.Sprintf("%d", tenantID), filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("media: open: %w", err)
	}
	return f, nil
}

// --- Handlers ---

// handleMediaUpload POST /api/media/upload
// Recibe multipart/form-data con campo "file". Valida tipo MIME y tamaño.
// Responde: { filename, url, type }.
func (s *Server) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	if s.mediaStore == nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "upload no habilitado")
		return
	}

	// Limitar el tamaño del body para evitar abusos.
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBytes)

	if err := r.ParseMultipartForm(maxMultipartBytes); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido: se esperaba multipart/form-data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "campo 'file' requerido")
		return
	}
	defer file.Close()

	// Detectar MIME type: primero Content-Type del header, luego sniff.
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		// Si no hay Content-Type en el part, intentar sniff de los primeros bytes.
		buf := make([]byte, 512)
		n, _ := io.ReadAtLeast(file, buf, 1)
		if n > 0 {
			mimeType = http.DetectContentType(buf[:n])
		}
		// Voltear al inicio para que Save lea todo.
		if seeker, ok := file.(io.Seeker); ok {
			seeker.Seek(0, io.SeekStart)
		}
	}

	// Normalizar: quitar parameters (ej. "image/jpeg; charset=...").
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	mediaType, ok := allowedMIMETypes[mimeType]
	if !ok {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"tipo de archivo no soportado. Usá JPG, PNG, GIF, WebP o MP4")
		return
	}

	// Validar tamaño por tipo.
	maxBytes := int64(maxUploadPhotoBytes)
	if mediaType == "video" {
		maxBytes = int64(maxUploadVideoBytes)
	}
	if header.Size > maxBytes {
		msg := fmt.Sprintf("el archivo excede %d MB", maxBytes/(1024*1024))
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", msg)
		return
	}

	// Generar nombre unico preservando la extension.
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		// Inferir extension del MIME type.
		switch mimeType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		case "video/mp4":
			ext = ".mp4"
		}
	}
	filename := uuid.New().String() + ext

	if _, err := s.mediaStore.Save(tenantID, filename, file); err != nil {
		slog.Error("media: save", "tenant", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo guardar el archivo")
		return
	}

	url := "/api/media/" + filename
	slog.Info("media: uploaded", "tenant", tenantID, "file", filename, "type", mediaType)

	respond(w, http.StatusOK, map[string]any{
		"filename": filename,
		"url":      url,
		"type":     mediaType,
	})
}

// handleMediaServe GET /api/media/{filename}
// Sirve el archivo multimedia desde el directorio del tenant.
func (s *Server) handleMediaServe(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	if s.mediaStore == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "upload no habilitado")
		return
	}

	filename := r.PathValue("filename")
	if filename == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "filename requerido")
		return
	}

	// Prevenir path traversal.
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "filename invalido")
		return
	}

	path, err := s.mediaStore.ServePath(tenantID, filename)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "archivo no encontrado")
		return
	}

	http.ServeFile(w, r, path)
}
