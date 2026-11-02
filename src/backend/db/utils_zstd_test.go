package db

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/klauspost/compress/zstd"
)

func compressRemoteJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithEncoderCRC(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := encoder.Close(); err != nil {
			t.Errorf("close test encoder: %v", err)
		}
	})
	return encoder.EncodeAll(data, nil)
}

func TestLoadAndUpdateFileCompressedSources(t *testing.T) {
	titles := []byte(`{"0100000000010000":{"id":"0100000000010000","name":"Game"}}`)
	versions := []byte(`{"0100000000010000":{"65536":"2026-11-02"}}`)
	for _, test := range []struct {
		name       string
		path       string
		fileName   string
		json       []byte
		compressed bool
		validate   JSONValidator
	}{
		{"plain titles", "/titles.json", "titles.json", titles, false, ValidateTitlesJSON},
		{"compressed titles", "/titles.json.zst", "titles.json", titles, true, ValidateTitlesJSON},
		{"extensionless compressed titles", "/download?format=zst", "titles.json", titles, true, ValidateTitlesJSON},
		{"plain JSON with zst extension", "/titles.json.zst", "titles.json", titles, false, ValidateTitlesJSON},
		{"compressed versions", "/versions.json.zst", "versions.json", versions, true, ValidateVersionsJSON},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := test.json
			if test.compressed {
				body = compressRemoteJSON(t, body)
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				// Deliberately avoid a compression MIME type: detection uses bytes.
				w.Header().Set("Content-Type", "application/octet-stream")
				if requests > 1 {
					if got := r.Header.Get("If-None-Match"); got != `"compressed-etag"` {
						t.Errorf("If-None-Match = %q", got)
					}
					w.Header().Set("ETag", `"refreshed-etag"`)
					w.WriteHeader(http.StatusNotModified)
					return
				}
				if got := r.Header.Get("If-None-Match"); got != "" {
					t.Errorf("first If-None-Match = %q", got)
				}
				w.Header().Set("ETag", `"compressed-etag"`)
				_, _ = w.Write(body)
			}))
			defer server.Close()
			url := server.URL + test.path
			path := filepath.Join(t.TempDir(), test.fileName)
			cache := RemoteFileCache{}
			for request := 0; request < 2; request++ {
				file, updated, err := LoadAndUpdateFile(url, path, cache, test.validate)
				if err != nil {
					t.Fatal(err)
				}
				contents, readErr := io.ReadAll(file)
				closeErr := file.Close()
				if readErr != nil || closeErr != nil || !bytes.Equal(contents, test.json) {
					t.Fatalf("local JSON = %q, read error = %v, close error = %v", contents, readErr, closeErr)
				}
				wantETag := `"compressed-etag"`
				if request > 0 {
					wantETag = `"refreshed-etag"`
				}
				if updated.URL != url || updated.ETag != wantETag || updated.SHA256 != hashBytes(test.json) {
					t.Fatalf("cache = %#v, want remote ETag and local JSON hash", updated)
				}
				cache = updated
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want 2", requests)
			}
		})
	}
}

func TestLoadAndUpdateFileCompressedFailuresPreserveLocalJSON(t *testing.T) {
	local := []byte(`{"0100000000010000":{"id":"0100000000010000","name":"Cached"}}`)
	compressed := compressRemoteJSON(t, local)
	badChecksum := bytes.Clone(compressed)
	badChecksum[len(badChecksum)-1] ^= 0xff
	for _, test := range []struct {
		name string
		body []byte
		want string
	}{
		{"corrupt frame", []byte("\x28\xb5\x2f\xfd\x08"), "decompress Zstandard JSON"},
		{"truncated header", compressed[:4], "decompress Zstandard JSON"},
		{"truncated frame", compressed[:len(compressed)-1], "decompress Zstandard JSON"},
		{"bad checksum", badChecksum, "decompress Zstandard JSON"},
		{"invalid JSON", compressRemoteJSON(t, []byte(`{"broken":`)), "validate downloaded JSON"},
		{"wrong JSON schema", compressRemoteJSON(t, []byte(`{"title":"Game"}`)), "validate downloaded JSON"},
		// A non-single-segment frame with a 128 MiB window must be rejected
		// before allocating its history, even though its payload is empty.
		{"oversized window", []byte("\x28\xb5\x2f\xfd\x00\x88\x01\x00\x00"), "window size exceeded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", "unusable")
				_, _ = w.Write(test.body)
			}))
			defer server.Close()
			for _, haveLocal := range []bool{false, true} {
				path := filepath.Join(t.TempDir(), "titles.json")
				cached := RemoteFileCache{}
				if haveLocal {
					if err := os.WriteFile(path, local, 0644); err != nil {
						t.Fatal(err)
					}
					cached = RemoteFileCache{URL: server.URL, ETag: "cached", SHA256: hashBytes(local)}
				}
				file, updated, err := LoadAndUpdateFile(server.URL, path, cached, ValidateTitlesJSON)
				if !haveLocal {
					if err == nil || !strings.Contains(err.Error(), test.want) || file != nil {
						t.Fatalf("without local file: file = %v, error = %v, want %q", file, err, test.want)
					}
					if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
						t.Fatalf("failed download created local file: %v", statErr)
					}
					continue
				}
				if err != nil {
					t.Fatalf("local fallback: %v", err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				contents, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(contents, local) || updated != cached {
					t.Fatalf("fallback = %q, cache = %#v, error = %v", contents, updated, err)
				}
			}
		})
	}
}

func TestReadRemoteJSONSizeLimits(t *testing.T) {
	const limit = 32
	for _, compressed := range []bool{false, true} {
		for _, size := range []int{limit - 1, limit, limit + 1} {
			json := bytes.Repeat([]byte("x"), size)
			body := json
			if compressed {
				body = compressRemoteJSON(t, json)
			}
			data, err := readRemoteJSON(bytes.NewReader(body), limit)
			if size > limit {
				if err == nil || !strings.Contains(err.Error(), "JSON exceeds 32-byte limit") || data != nil {
					t.Fatalf("compressed=%v size=%d: data=%q, error=%v", compressed, size, data, err)
				}
			} else if err != nil || !bytes.Equal(data, json) {
				t.Fatalf("compressed=%v size=%d: data=%q, error=%v", compressed, size, data, err)
			}
		}
	}
}

func TestReadRemoteJSONFrameSequences(t *testing.T) {
	json := []byte(`{"title":"Game"}`)
	first := compressRemoteJSON(t, json[:5])
	second := compressRemoteJSON(t, json[5:])
	concatenated := append(bytes.Clone(first), second...)
	skippable := append([]byte("\x50\x2a\x4d\x18\x04\x00\x00\x00skip"), compressRemoteJSON(t, json)...)
	for _, body := range [][]byte{concatenated, skippable, []byte("{}"), nil} {
		data, err := readRemoteJSON(bytes.NewReader(body), 32)
		want := json
		if len(body) < 4 {
			want = body
		}
		if err != nil || !bytes.Equal(data, want) {
			t.Fatalf("decoded = %q, error = %v, want %q", data, err, want)
		}
	}
	if _, err := readRemoteJSON(bytes.NewReader(concatenated), int64(len(json)-1)); err == nil {
		t.Fatal("concatenated frames bypassed the decoded size limit")
	}
}

func TestReadRemoteJSONReadErrors(t *testing.T) {
	want := errors.New("synthetic read failure")
	for _, reader := range []io.Reader{
		iotest.ErrReader(want),
		io.MultiReader(strings.NewReader(`{"title":"Game"}`), iotest.ErrReader(want)),
		io.MultiReader(bytes.NewReader(compressRemoteJSON(t, []byte(`{"title":"Game"}`))), iotest.ErrReader(want)),
	} {
		if _, err := readRemoteJSON(reader, 1024); !errors.Is(err, want) {
			t.Fatalf("error = %v, want wrapped read failure", err)
		}
	}
}
