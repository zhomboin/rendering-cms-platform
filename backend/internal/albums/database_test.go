package albums

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

type testObjects struct {
	mu        sync.Mutex
	objects   map[string][]byte
	deleteErr error
	deleted   int
	afterPut  func()
}

func (s *testObjects) PutObject(ctx context.Context, key string, body []byte, contentType, cacheControl string) error {
	s.mu.Lock()
	s.objects[key] = append([]byte{}, body...)
	hook := s.afterPut
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (s *testObjects) DeleteObject(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, key)
	return nil
}

type albumFixture struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
	handler Handler
	objects *testObjects
	id      pgtype.UUID
}

func albumDatabaseFixture(t *testing.T) albumFixture {
	t.Helper()
	pool := testutil.Database(t)
	q := dbgen.New(pool)
	testutil.Exec(t, pool, "insert into users(user_id,email,name,password_hash) values($1,'fixture@example.invalid','测试','synthetic')", testUserID)
	album, err := q.CreatePhotoAlbum(context.Background(), dbgen.CreatePhotoAlbumParams{
		Slug: "test-album", Title: "相册", Description: "描述", Tags: []string{},
		CreatedBy: uuidForTest(testUserID),
	})
	if err != nil {
		t.Fatal(err)
	}
	objects := &testObjects{objects: make(map[string][]byte)}
	return albumFixture{
		pool:    pool,
		queries: q,
		objects: objects,
		id:      album.AlbumID,
		handler: NewHandlerWithOptions(NewDatabaseStore(pool), objects, HandlerOptions{
			PublicBaseURL: "https://images.example.invalid",
		}),
	}
}

func (f albumFixture) upload(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	authenticatedAlbumRouter(t, f.handler).ServeHTTP(response, multipartPhotoRequest(t,
		"/albums/"+f.id.String()+"/photos", map[string]string{"alt": "test"},
		"test.png", encodeTestPNG(t, 2, 2), "image/png"))
	return response
}

func (f albumFixture) singlePhoto(t *testing.T) dbgen.GetAlbumPhotoByIDRow {
	t.Helper()
	if result := f.upload(t); result.Code != 201 {
		t.Fatalf("seed photo: %d %s", result.Code, result.Body.String())
	}
	rows, err := f.queries.ListAlbumPhotos(context.Background(), f.id)
	if err != nil || len(rows) != 1 {
		t.Fatalf("photos: %v %v", rows, err)
	}
	photo, err := f.queries.GetAlbumPhotoByID(context.Background(), dbgen.GetAlbumPhotoByIDParams{
		AlbumID: f.id, PhotoID: rows[0].PhotoID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return photo
}

func (f albumFixture) count(t *testing.T, table string) int {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(context.Background(), "select count(*) from "+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func (f albumFixture) injectFailure(t *testing.T, table, operation string) {
	t.Helper()
	testutil.Exec(t, f.pool, "create function injected_failure() returns trigger language plpgsql as $$ begin raise exception 'injected failure'; end $$")
	testutil.Exec(t, f.pool, "create trigger injected_failure before "+operation+" on "+table+" for each row execute function injected_failure()")
}

var errTestObjectDelete = errors.New("injected object deletion failure")
