package albums

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

func paddedPNG(t *testing.T, size int) ([]byte, []byte) {
	t.Helper()
	original := encodeTestPNG(t, 2, 2)
	payload := bytes.Repeat([]byte("x"), size-len(original)-12)
	copy(payload, []byte("Comment\x00private metadata"))
	chunk := make([]byte, len(payload)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(payload)))
	copy(chunk[4:8], "tEXt")
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	result := append([]byte{}, original[:len(original)-12]...)
	result = append(result, chunk...)
	return append(result, original[len(original)-12:]...), original
}

func TestPhotoUploadLimitsAndTemporaryCleanup(t *testing.T) {
	const limit = 50 * 1024 * 1024
	for _, tc := range []struct {
		name       string
		size       int
		alt        string
		truncate   bool
		want       int
		fullRouter bool
		unknown    bool
	}{
		{"exact default router", limit, "photo", false, 201, true, false},
		{"exact chunked", limit, "photo", false, 201, true, true},
		{"over file limit", limit + 1, "photo", false, 400, true, false},
		{"empty", 0, "photo", false, 400, true, false},
		{"later validation", limit, "", false, 400, false, false},
		{"parse failure", limit, "photo", true, 400, false, false},
		{"handler total limit", 52 * 1024 * 1024, "photo", false, 413, false, false},
		{"handler chunked total limit", 52 * 1024 * 1024, "photo", false, 413, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Setenv("TMPDIR", tempDir)
			store := &albumStoreStub{albumByID: dbgen.GetPhotoAlbumByIDRow{AlbumID: uuidForTest(testAlbumID)}}
			handler, objects := newUploadHandler(t, store)
			var router http.Handler = authenticatedAlbumRouter(t, handler)
			target := "/albums/" + testAlbumID + "/photos"
			if tc.fullRouter {
				router = httpapi.NewRouter(httpapi.WithJWTSecret("test-secret"), httpapi.WithAdminRoutes(handler.RegisterAdminRoutes))
				target = "/api/v1/admin" + target
			}
			var content, original []byte
			if tc.size > 0 {
				content, original = paddedPNG(t, tc.size)
			}
			req := multipartPhotoRequest(t, target, map[string]string{"alt": tc.alt}, "photo.png", content, "image/png")
			if tc.truncate {
				body, _ := io.ReadAll(req.Body)
				body = body[:len(body)-10]
				req.Body = io.NopCloser(bytes.NewReader(body))
				req.ContentLength = int64(len(body))
			}
			if tc.unknown {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status=%d want %d body=%s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == 201 {
				if len(objects.putCalls) != 1 {
					t.Fatal("upload never reached storage")
				}
				if !bytes.Equal(objects.putCalls[0].body, original) {
					t.Error("metadata not stripped")
				}
				if store.lastAssetCreate.ByteSize != int32(len(original)) || store.lastPhotoCreate.Width != 2 {
					t.Error("database did not receive cleaned image metadata")
				}
			} else if len(objects.putCalls) != 0 {
				t.Error("invalid upload reached storage")
			}
			entries, err := os.ReadDir(tempDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("multipart temporary files leaked: %v", entries)
			}
		})
	}
}
