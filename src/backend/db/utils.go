package db

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

const (
	maxRemoteJSONSize   = 512 << 20
	maxRemoteZstdWindow = 64 << 20
)

type ProgressUpdater interface {
	UpdateProgress(curr int, total int, message string)
}

// JSONValidator checks that downloaded bytes have the shape expected by the
// caller before they can replace the local data file.
type JSONValidator func([]byte) error

// RemoteFileCache contains the validator metadata for one remotely sourced file.
type RemoteFileCache struct {
	URL    string
	ETag   string
	SHA256 string
}

// LoadAndUpdateFile downloads or opens filePath and returns an open file.
// The caller owns the returned file and must close it.
func LoadAndUpdateFile(url string, filePath string, cached RemoteFileCache, validate JSONValidator) (*os.File, RemoteFileCache, error) {
	if validate == nil {
		return nil, RemoteFileCache{}, errors.New("JSON validator is required")
	}

	_, localHash, localAvailable, err := readValidatedLocalFile(filePath, validate)
	if err != nil {
		return nil, RemoteFileCache{}, err
	}

	requestETag := ""
	cacheMatchesLocal := localAvailable && cached.URL == url && cached.ETag != "" && cached.SHA256 == localHash
	if cacheMatchesLocal {
		requestETag = cached.ETag
	}

	body, newETag, notModified, downloadErr := downloadBytesFromURL(url, requestETag)
	if downloadErr == nil && notModified {
		// Recheck the file after the request. Another process or the user may
		// have replaced it while the conditional request was in flight.
		_, currentHash, currentAvailable, readErr := readValidatedLocalFile(filePath, validate)
		if readErr == nil && requestETag != "" && currentAvailable && currentHash == cached.SHA256 {
			if newETag != "" {
				cached.ETag = newETag
			}
			file, openErr := os.Open(filePath)
			if openErr != nil {
				return nil, RemoteFileCache{}, fmt.Errorf("open unchanged local data file %q: %w", filePath, openErr)
			}
			return file, cached, nil
		}

		// A 304 without a matching local copy cannot be used. Retry once
		// without a validator so even unusual servers cannot strand a new install.
		body, newETag, notModified, downloadErr = downloadBytesFromURL(url, "")
		cacheMatchesLocal = false
	}
	if downloadErr == nil && notModified {
		downloadErr = errors.New("server returned 304 without a matching local data file")
	}

	if downloadErr == nil {
		if err := validate(body); err != nil {
			downloadErr = fmt.Errorf("validate downloaded JSON: %w", err)
		} else if err := replaceFileAtomically(filePath, body); err != nil {
			downloadErr = fmt.Errorf("save downloaded data file %q: %w", filePath, err)
		} else {
			file, err := os.Open(filePath)
			if err != nil {
				return nil, RemoteFileCache{}, fmt.Errorf("open downloaded data file %q: %w", filePath, err)
			}
			return file, RemoteFileCache{URL: url, ETag: newETag, SHA256: hashBytes(body)}, nil
		}
	}

	zap.S().Infof("file [%v] was not downloaded, reason - [%v]", url, downloadErr)
	_, localHash, localAvailable, localErr := readValidatedLocalFile(filePath, validate)
	if localErr != nil {
		return nil, RemoteFileCache{}, errors.Join(fmt.Errorf("download %q: %w", url, downloadErr), localErr)
	}
	if !localAvailable {
		return nil, RemoteFileCache{}, fmt.Errorf("download failed and no usable local data file is available: %w", downloadErr)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, RemoteFileCache{}, fmt.Errorf("open local data file %q after download failure: %w", filePath, err)
	}
	resultCache := RemoteFileCache{URL: url, SHA256: localHash}
	if cacheMatchesLocal && cached.URL == url && cached.SHA256 == localHash {
		resultCache.ETag = cached.ETag
	}
	return file, resultCache, nil
}

func readValidatedLocalFile(filePath string, validate JSONValidator) ([]byte, string, bool, error) {
	info, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("check local data file %q: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, "", false, fmt.Errorf("local data path %q is a directory", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("read local data file %q: %w", filePath, err)
	}
	if len(data) == 0 || validate(data) != nil {
		return data, "", false, nil
	}
	return data, hashBytes(data), true, nil
}

func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func downloadBytesFromURL(url string, etag string) ([]byte, string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", false, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
	}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	responseETag := resp.Header.Get("ETag")
	if resp.StatusCode == http.StatusNotModified {
		return nil, responseETag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("got HTTP status %s", resp.Status)
	}
	body, err := readRemoteJSON(resp.Body, maxRemoteJSONSize)
	if err != nil {
		return nil, "", false, err
	}
	return body, responseETag, false, nil
}

// readRemoteJSON detects compression from the body rather than the URL or MIME
// type. Both downloaded formats become ordinary JSON before validation/caching.
func readRemoteJSON(source io.Reader, maxSize int64) ([]byte, error) {
	buffered := bufio.NewReader(source)
	signature, err := buffered.Peek(4)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("inspect downloaded data: %w", err)
	}

	var reader io.Reader = buffered
	readOperation := "read downloaded JSON"
	// A Zstandard stream can start with a normal frame or a skippable frame.
	isZstandard := len(signature) == 4 && (string(signature) == "\x28\xb5\x2f\xfd" ||
		(signature[0]&0xf0 == 0x50 && string(signature[1:]) == "\x2a\x4d\x18"))
	if isZstandard {
		decoder, err := zstd.NewReader(buffered,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderMaxMemory(maxRemoteZstdWindow),
		)
		if err != nil {
			return nil, fmt.Errorf("initialize Zstandard decoder: %w", err)
		}
		defer decoder.Close()
		reader = decoder
		readOperation = "decompress Zstandard JSON"
	}

	// The streaming decoder bounds its window, not the total decoded output.
	// Read one extra byte to distinguish a complete file from an oversized one.
	data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", readOperation, err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%s: JSON exceeds %d-byte limit", readOperation, maxSize)
	}
	return data, nil
}

func replaceFileAtomically(fileName string, data []byte) error {
	directory := filepath.Dir(fileName)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(fileName)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if err := temporary.Chmod(0644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryName, fileName); err != nil {
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}
