package attachment

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageAcceptsValidatedImages(t *testing.T) {
	storage := testStorage(t, 1024*1024)
	pngData := encodePNG(t)
	jpegData := encodeJPEG(t)
	webpData, err := base64.StdEncoding.DecodeString(
		"UklGRkoAAABXRUJQVlA4ID4AAADQAQCdASoBAAEAAUAmJaQAA3AA/v89WAAAAA==",
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		filename  string
		data      []byte
		mediaType string
	}{
		{name: "png", filename: "proof.png", data: pngData, mediaType: "image/png"},
		{name: "jpeg", filename: "proof.jpeg", data: jpegData, mediaType: "image/jpeg"},
		{name: "webp", filename: "proof.webp", data: webpData, mediaType: "image/webp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := multipartHeader(t, test.filename, test.data)
			upload, err := storage.Stage(header)
			if err != nil {
				t.Fatalf("stage image: %v", err)
			}
			t.Cleanup(func() { storage.RemoveTemp(upload.TempPath) })
			if upload.MediaType != test.mediaType {
				t.Fatalf("media type = %q, want %q", upload.MediaType, test.mediaType)
			}
			if upload.Width == 0 || upload.Height == 0 {
				t.Fatalf("invalid dimensions %dx%d", upload.Width, upload.Height)
			}
			if len(upload.SHA256) != 32 {
				t.Fatalf("sha256 length = %d", len(upload.SHA256))
			}
		})
	}
}

func TestStageRejectsSpoofedAndOversizedFilesWithoutLeavingTempFiles(t *testing.T) {
	storage := testStorage(t, 64)
	tests := []struct {
		name     string
		filename string
		data     []byte
	}{
		{name: "svg disguised as png", filename: "proof.png", data: []byte(`<svg/>`)},
		{name: "mismatched extension", filename: "proof.jpg", data: encodePNG(t)},
		{name: "oversized", filename: "proof.png", data: bytes.Repeat([]byte("x"), 65)},
		{name: "unsupported extension", filename: "proof.gif", data: encodePNG(t)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := storage.Stage(multipartHeader(t, test.filename, test.data)); err == nil {
				t.Fatal("expected validation failure")
			}
			entries, err := os.ReadDir(filepath.Join(storage.root, ".tmp"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("temporary files left behind: %d", len(entries))
			}
		})
	}
}

func TestFinalPathRejectsEscapes(t *testing.T) {
	storage := testStorage(t, 1024)
	for _, value := range []string{"../outside.png", "/tmp/outside.png", ""} {
		if _, err := storage.FinalPath(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	path, err := storage.FinalPath("evaluations/run/result/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, storage.root+string(filepath.Separator)) {
		t.Fatalf("path escaped root: %s", path)
	}
}

func testStorage(t *testing.T, maxSize int64) Storage {
	t.Helper()
	storage, err := NewStorage(
		t.TempDir(), maxSize, []string{"image/png", "image/jpeg", "image/webp"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return storage
}

func multipartHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(2 << 20); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { request.MultipartForm.RemoveAll() })
	return request.MultipartForm.File["files"][0]
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, testImage()); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func testImage() image.Image {
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 200, G: 20, B: 20, A: 255})
	return value
}
