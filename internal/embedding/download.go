package embedding

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	hfBaseURL  = "https://huggingface.co"
	cacheDir   = ".cache/bragi/models"
)

// DownloadProgress reports model download progress.
type DownloadProgress struct {
	File            string
	BytesDownloaded int64
	TotalBytes      int64
	Done            bool
	Err             error
}

// ModelCacheDir returns the local cache directory for a given model name.
// E.g., "nomic-ai/nomic-embed-text-v1.5" → "~/.cache/bragi/models/nomic-ai--nomic-embed-text-v1.5"
func ModelCacheDir(modelName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	safeName := strings.ReplaceAll(modelName, "/", "--")
	return filepath.Join(home, cacheDir, safeName), nil
}

// EnsureModel checks if the model is cached locally, downloading it if not.
// Progress is reported on the channel (optional, can be nil).
// Returns the path to the model directory.
func EnsureModel(modelName string, progress chan<- DownloadProgress) (string, error) {
	dir, err := ModelCacheDir(modelName)
	if err != nil {
		return "", err
	}

	// Check if already downloaded by looking for vocab.txt and an ONNX file.
	vocabPath := filepath.Join(dir, "vocab.txt")
	if _, err := os.Stat(vocabPath); err == nil {
		// Check for any .onnx file.
		if hasONNXFile(dir) {
			return dir, nil
		}
	}

	// Download required files.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	files := []string{
		"onnx/model_quantized.onnx",
		"vocab.txt",
		"tokenizer_config.json",
	}

	for _, file := range files {
		url := fmt.Sprintf("%s/%s/resolve/main/%s", hfBaseURL, modelName, file)

		// Determine local path (flatten onnx/ prefix).
		localName := filepath.Base(file)
		localPath := filepath.Join(dir, localName)

		if err := downloadFile(url, localPath, file, progress); err != nil {
			return "", fmt.Errorf("downloading %s: %w", file, err)
		}
	}

	if progress != nil {
		progress <- DownloadProgress{Done: true}
	}

	return dir, nil
}

// downloadFile downloads a single file from a URL to a local path.
func downloadFile(url, destPath, name string, progress chan<- DownloadProgress) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer out.Close()

	if progress != nil {
		// Wrap reader for progress reporting.
		reader := &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			file:       name,
			progressCh: progress,
		}
		_, err = io.Copy(out, reader)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	return err
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	file       string
	progressCh chan<- DownloadProgress
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	pr.progressCh <- DownloadProgress{
		File:            pr.file,
		BytesDownloaded: pr.downloaded,
		TotalBytes:      pr.total,
	}

	return n, err
}

// ModelCached returns true when the model directory already contains a vocab
// file and at least one ONNX model file.  Pass the result of ModelCacheDir.
func ModelCached(dir string) bool {
	vocabPath := filepath.Join(dir, "vocab.txt")
	if _, err := os.Stat(vocabPath); err != nil {
		return false
	}
	return hasONNXFile(dir)
}

// hasONNXFile checks if any .onnx file exists in the directory.
func hasONNXFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".onnx") {
			return true
		}
	}
	return false
}
