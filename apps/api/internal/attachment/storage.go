package attachment

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
)

var (
	ErrUnsupportedMediaType = fmt.Errorf("unsupported image media type")
	ErrFileTooLarge         = fmt.Errorf("image size exceeds limit")
)

type Storage struct {
	root             string
	maxFileSize      int64
	allowedMediaType map[string]bool
}

func NewStorage(root string, maxFileSize int64, allowed []string) (Storage, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Storage{}, err
	}
	if err := os.MkdirAll(filepath.Join(absolute, ".tmp"), 0o750); err != nil {
		return Storage{}, fmt.Errorf("create upload temp directory: %w", err)
	}
	mediaTypes := make(map[string]bool, len(allowed))
	for _, value := range allowed {
		mediaTypes[value] = true
	}
	return Storage{root: absolute, maxFileSize: maxFileSize, allowedMediaType: mediaTypes}, nil
}

func (storage Storage) Stage(fileHeader *multipart.FileHeader) (Upload, error) {
	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
		return Upload{}, ErrUnsupportedMediaType
	}
	source, err := fileHeader.Open()
	if err != nil {
		return Upload{}, err
	}
	defer source.Close()
	temp, err := os.CreateTemp(filepath.Join(storage.root, ".tmp"), "upload-*")
	if err != nil {
		return Upload{}, err
	}
	tempPath := temp.Name()
	defer func() {
		temp.Close()
	}()
	hash := sha256.New()
	limited := io.LimitReader(source, storage.maxFileSize+1)
	size, err := io.Copy(io.MultiWriter(temp, hash), limited)
	if err != nil {
		os.Remove(tempPath)
		return Upload{}, err
	}
	if size == 0 || size > storage.maxFileSize {
		os.Remove(tempPath)
		if size > storage.maxFileSize {
			return Upload{}, ErrFileTooLarge
		}
		return Upload{}, ErrUnsupportedMediaType
	}
	if err := temp.Sync(); err != nil {
		os.Remove(tempPath)
		return Upload{}, err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return Upload{}, err
	}
	mediaType, width, height, err := inspectImage(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return Upload{}, ErrUnsupportedMediaType
	}
	if !storage.allowedMediaType[mediaType] || !extensionMatches(extension, mediaType) {
		os.Remove(tempPath)
		return Upload{}, ErrUnsupportedMediaType
	}
	originalName := filepath.Base(strings.TrimSpace(fileHeader.Filename))
	if originalName == "" || originalName == "." {
		originalName = "image" + extension
	}
	return Upload{
		TempPath: tempPath, OriginalName: originalName, MediaType: mediaType,
		Extension: canonicalExtension(mediaType), FileSize: size,
		SHA256: hash.Sum(nil), Width: width, Height: height,
	}, nil
}

func (storage Storage) FinalPath(relative string) (string, error) {
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("unsafe storage path")
	}
	path := filepath.Join(storage.root, filepath.FromSlash(relative))
	relativeToRoot, err := filepath.Rel(storage.root, path)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage path escapes upload root")
	}
	return path, nil
}

func (storage Storage) Move(upload Upload, relative string) error {
	finalPath, err := storage.FinalPath(relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return err
	}
	return os.Rename(upload.TempPath, finalPath)
}

func (storage Storage) RemoveRelative(relative string) {
	path, err := storage.FinalPath(relative)
	if err == nil {
		os.Remove(path)
	}
}

func (storage Storage) RemoveTemp(path string) {
	if path != "" {
		os.Remove(path)
	}
}

func (storage Storage) Open(relative string) (*os.File, error) {
	path, err := storage.FinalPath(relative)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func inspectImage(path string) (string, uint32, uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", 0, 0, fmt.Errorf("invalid image content")
	}
	switch format {
	case "png":
		return "image/png", uint32(config.Width), uint32(config.Height), nil
	case "jpeg":
		return "image/jpeg", uint32(config.Width), uint32(config.Height), nil
	case "webp":
		return "image/webp", uint32(config.Width), uint32(config.Height), nil
	default:
		return "", 0, 0, fmt.Errorf("unsupported image content")
	}
}

func extensionMatches(extension, mediaType string) bool {
	switch mediaType {
	case "image/png":
		return extension == ".png"
	case "image/jpeg":
		return extension == ".jpg" || extension == ".jpeg"
	case "image/webp":
		return extension == ".webp"
	default:
		return false
	}
}

func canonicalExtension(mediaType string) string {
	switch mediaType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	default:
		return ".webp"
	}
}
