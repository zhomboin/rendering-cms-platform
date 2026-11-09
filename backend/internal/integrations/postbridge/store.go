package postbridge

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

var (
	ErrStoreNotFound           = errors.New("PostBridge record not found")
	ErrStoreBusy               = errors.New("PostBridge record is leased")
	ErrStoreConnectionMismatch = errors.New("PostBridge connection does not match the stored identity")
)

type Store struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:    pool,
		queries: dbgen.New(pool),
	}
}

type ArticleSource struct {
	ID            string
	Title         string
	Summary       string
	Body          string
	CoverImageURL string
	ArticleName   string
	Status        string
	Tags          []string
	Version       int64
}

type AttemptRecord struct {
	ID                     string
	ArticleID              string
	SourceID               string
	RequestID              string
	TargetAccountID        string
	AuthorizationID        string
	Status                 string
	LeaseToken             string
	SourceVersion          int64
	StateSeq               int64
	AuthorizationExpiresAt time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	RevokedAt              *time.Time
	CancelPending          bool
	Data                   []byte
}

type EventRecord struct {
	ID         string
	LeaseToken string
	Body       []byte
	Snapshot   []byte
	Attempts   int32
}

func (s *Store) CreateAttempt(ctx context.Context, sourceID, articleID, requestID string, build func(ArticleSource, int64, func([]string) error) (AttemptRecord, error)) (AttemptRecord, error) {
	articleKey, err := parsePostBridgeArticleID(articleID)
	if err != nil {
		return AttemptRecord{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AttemptRecord{}, err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	key := dbgen.GetPostBridgeAttemptByRequestParams{
		SourceID:  sourceID,
		ArticleID: articleKey,
		RequestID: requestID,
	}
	if existing, err := q.GetPostBridgeAttemptByRequest(ctx, key); !errors.Is(err, pgx.ErrNoRows) {
		return attemptRecord(existing, err)
	}

	article, err := q.LockPostBridgeArticle(ctx, articleKey)
	if err != nil {
		return AttemptRecord{}, storeError(err)
	}

	// Concurrent preparation requests serialize on the article row. Check again
	// after locking so a replay never rebuilds a saved request.
	if existing, err := q.GetPostBridgeAttemptByRequest(ctx, key); !errors.Is(err, pgx.ErrNoRows) {
		return attemptRecord(existing, err)
	}

	version := int64(article.Version)
	seq, err := q.EnsurePostBridgeArticleState(ctx, dbgen.EnsurePostBridgeArticleStateParams{
		SourceID:      sourceID,
		ArticleID:     articleKey,
		SourceVersion: version,
	})
	if err != nil {
		return AttemptRecord{}, err
	}
	attempt, err := build(ArticleSource{
		ID:            uuid.UUID(article.ArticleID.Bytes).String(),
		Title:         article.Title,
		Summary:       article.Summary,
		Body:          article.BodyMdx,
		CoverImageURL: article.CoverImageUrl.String,
		ArticleName:   article.ArticleName,
		Status:        string(article.Status),
		Tags:          article.Tags,
		Version:       version,
	}, seq, func(urls []string) error {
		if len(urls) != 0 {
			if err := q.LockPostBridgeImageProvenance(ctx); err != nil {
				return err
			}
		}
		return checkPublishedImages(ctx, q, urls)
	})
	if err != nil {
		return AttemptRecord{}, err
	}
	created, err := q.CreatePostBridgeAttempt(ctx, dbgen.CreatePostBridgeAttemptParams{
		ID:              attempt.ID,
		SourceID:        sourceID,
		ArticleID:       articleKey,
		RequestID:       requestID,
		SourceVersion:   version,
		StateSeq:        seq,
		TargetAccountID: attempt.TargetAccountID,
		AuthorizationID: attempt.AuthorizationID,
		AuthorizationExpiresAt: pgtype.Timestamptz{
			Time:  attempt.AuthorizationExpiresAt,
			Valid: true,
		},
		Status: attempt.Status,
		Data:   attempt.Data,
	})
	if err != nil {
		return AttemptRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptRecord{}, err
	}
	return attemptRecord(created, nil)
}

func checkPublishedImages(ctx context.Context, q *dbgen.Queries, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	approved, err := q.AllPostBridgeImageURLsPublished(ctx, urls)
	if err == nil && !approved {
		return ErrUnapprovedImage
	}
	return err
}

func (s *Store) GetAttempt(ctx context.Context, id string) (AttemptRecord, error) {
	return attemptRecord(s.queries.GetPostBridgeAttempt(ctx, id))
}

func (s *Store) GetAttemptByRequest(ctx context.Context, sourceID, articleID, requestID string) (AttemptRecord, error) {
	articleKey, err := parsePostBridgeArticleID(articleID)
	if err != nil {
		return AttemptRecord{}, err
	}
	return attemptRecord(s.queries.GetPostBridgeAttemptByRequest(ctx, dbgen.GetPostBridgeAttemptByRequestParams{
		SourceID:  sourceID,
		ArticleID: articleKey,
		RequestID: requestID,
	}))
}

func (s *Store) ListAttempts(ctx context.Context, sourceID, articleID, status string, page, pageSize int32) ([]AttemptRecord, int64, error) {
	var articleKey pgtype.UUID
	if articleID != "" {
		key, err := parsePostBridgeArticleID(articleID)
		if err != nil {
			return nil, 0, err
		}
		articleKey = key
	}
	if page < 1 || pageSize < 1 {
		return nil, 0, errors.New("invalid PostBridge pagination")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	rows, err := q.ListPostBridgeAttempts(ctx, dbgen.ListPostBridgeAttemptsParams{
		SourceID:   sourceID,
		ArticleID:  articleKey,
		Status:     status,
		PageOffset: int64(page-1) * int64(pageSize),
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountPostBridgeAttempts(ctx, dbgen.CountPostBridgeAttemptsParams{
		SourceID:  sourceID,
		ArticleID: articleKey,
		Status:    status,
	})
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	result := make([]AttemptRecord, 0, len(rows))
	for _, row := range rows {
		item, _ := attemptRecord(row, nil)
		result = append(result, item)
	}
	return result, total, nil
}

func (s *Store) ClaimAttempt(ctx context.Context, id string) (AttemptRecord, error) {
	row, err := s.queries.ClaimPostBridgeAttempt(ctx, id)
	if !errors.Is(err, pgx.ErrNoRows) {
		return attemptRecord(row, err)
	}
	if _, err := s.GetAttempt(ctx, id); err != nil {
		return AttemptRecord{}, err
	}
	return AttemptRecord{}, ErrStoreBusy
}

func (s *Store) ClaimNextAttempt(ctx context.Context, sourceID string) (AttemptRecord, error) {
	return attemptRecord(s.queries.ClaimNextPostBridgeAttempt(ctx, sourceID))
}

func (s *Store) SaveAttempt(ctx context.Context, a AttemptRecord, next *time.Time, release bool) (AttemptRecord, error) {
	row, err := s.queries.SavePostBridgeAttempt(ctx, dbgen.SavePostBridgeAttemptParams{
		ID:                 a.ID,
		LeaseToken:         a.LeaseToken,
		Status:             a.Status,
		Data:               a.Data,
		RevocationObserved: a.RevokedAt != nil,
		CancelPending:      a.CancelPending,
		NextSyncAt:         nullablePostBridgeTime(next),
		ReleaseLease:       release,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return AttemptRecord{}, ErrStoreBusy
	}
	return attemptRecord(row, err)
}

func (s *Store) RevokeAttempt(ctx context.Context, id string) (AttemptRecord, error) {
	return s.revokeAttempt(ctx, id, "")
}

func (s *Store) revokeAttempt(ctx context.Context, id, expectedLease string) (AttemptRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttemptRecord{}, err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	existing, err := q.GetPostBridgeAttempt(ctx, id)
	if err != nil {
		return attemptRecord(existing, err)
	}
	if expectedLease != "" && existing.LeaseToken != expectedLease {
		return AttemptRecord{}, ErrStoreBusy
	}
	if existing.RevokedAt.Valid {
		return attemptRecord(existing, nil)
	}

	// Take the source lock before touching the attempt, matching article writers.
	article, err := q.LockPostBridgeArticle(ctx, existing.ArticleID)
	status, version := "deleted", existing.SourceVersion
	if err == nil {
		status, version = string(article.Status), int64(article.Version)
		if status == "archived" {
			status = "draft"
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AttemptRecord{}, err
	}
	row, err := q.RevokePostBridgeAttempt(ctx, dbgen.RevokePostBridgeAttemptParams{
		ID:            id,
		ExpectedLease: expectedLease,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, err := q.GetPostBridgeAttempt(ctx, id)
		if err == nil && expectedLease != "" && (current.LeaseToken != expectedLease || !current.RevokedAt.Valid) {
			return AttemptRecord{}, ErrStoreBusy
		}
		return attemptRecord(current, err)
	}
	if err != nil {
		return AttemptRecord{}, err
	}
	if err := q.EnqueuePostBridgeRevocation(ctx, dbgen.EnqueuePostBridgeRevocationParams{
		SourceID:        row.SourceID,
		ArticleID:       row.ArticleID,
		SourceVersion:   version,
		ArticleStatus:   status,
		AuthorizationID: row.AuthorizationID,
	}); err != nil {
		return AttemptRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptRecord{}, err
	}
	return attemptRecord(row, nil)
}

func (s *Store) HasPendingArticleEvents(ctx context.Context, sourceID, articleID string, beforeStateSeq int64) (bool, error) {
	articleKey, err := parsePostBridgeArticleID(articleID)
	if err != nil {
		return false, err
	}
	return s.queries.HasPendingPostBridgeArticleEvents(ctx, dbgen.HasPendingPostBridgeArticleEventsParams{
		SourceID:  sourceID,
		ArticleID: articleKey,
		StateSeq:  beforeStateSeq,
	})
}

func (s *Store) ClaimEvent(ctx context.Context, sourceID string) (EventRecord, error) {
	event, err := s.queries.ClaimPostBridgeEvent(ctx, sourceID)
	if err != nil {
		return EventRecord{}, storeError(err)
	}
	return EventRecord{
		ID:         event.ID,
		LeaseToken: event.LeaseToken,
		Body:       event.Body,
		Snapshot:   event.Snapshot,
		Attempts:   event.Attempts,
	}, nil
}

func (s *Store) FreezeEvent(ctx context.Context, event EventRecord, body []byte) error {
	count, err := s.queries.FreezePostBridgeEvent(ctx, dbgen.FreezePostBridgeEventParams{
		ID:         event.ID,
		LeaseToken: event.LeaseToken,
		Body:       body,
	})
	return eventWriteError(count, err)
}

func (s *Store) FinishEvent(ctx context.Context, event EventRecord, next *time.Time, errorCode string) error {
	count, err := s.queries.FinishPostBridgeEvent(ctx, dbgen.FinishPostBridgeEventParams{
		ID:            event.ID,
		LeaseToken:    event.LeaseToken,
		NextAttemptAt: nullablePostBridgeTime(next),
		ErrorCode:     errorCode,
	})
	return eventWriteError(count, err)
}

func (s *Store) BindConnection(ctx context.Context, sourceID, environment, origin, clientID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	if err := q.InsertPostBridgeConnection(ctx, dbgen.InsertPostBridgeConnectionParams{
		SourceID:    sourceID,
		Environment: environment,
		Origin:      origin,
		ClientID:    clientID,
	}); err != nil {
		return err
	}
	connection, err := q.GetPostBridgeConnection(ctx)
	if err != nil {
		return storeError(err)
	}
	if connection.SourceID != sourceID || connection.Environment != environment || connection.Origin != origin || connection.ClientID != clientID {
		return ErrStoreConnectionMismatch
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveRegistration(ctx context.Context, sourceID, environment string, registration Registration) error {
	return s.queries.SavePostBridgeRegistration(ctx, dbgen.SavePostBridgeRegistrationParams{
		SourceID:       sourceID,
		Environment:    environment,
		RegistrationID: registration.ID,
		Status:         registration.Status,
	})
}

func (s *Store) GetRegistration(ctx context.Context, sourceID, environment string) (Registration, error) {
	registration, err := s.queries.GetPostBridgeRegistration(ctx, dbgen.GetPostBridgeRegistrationParams{
		SourceID:    sourceID,
		Environment: environment,
	})
	if err != nil {
		return Registration{}, storeError(err)
	}
	return Registration{
		ID:     registration.RegistrationID,
		Status: registration.Status,
	}, nil
}

func parsePostBridgeArticleID(raw string) (pgtype.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return pgtype.UUID{}, ErrStoreNotFound
	}
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}, nil
}

func nullablePostBridgeTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
	}
}

func storeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStoreNotFound
	}
	return err
}

func eventWriteError(count int64, err error) error {
	if err == nil && count == 0 {
		return ErrStoreBusy
	}
	return err
}

func attemptRecord(row dbgen.PostbridgeAttempt, err error) (AttemptRecord, error) {
	if err != nil {
		return AttemptRecord{}, storeError(err)
	}
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		revokedAt = &row.RevokedAt.Time
	}
	return AttemptRecord{
		ID:                     row.ID,
		ArticleID:              uuid.UUID(row.ArticleID.Bytes).String(),
		SourceID:               row.SourceID,
		RequestID:              row.RequestID,
		TargetAccountID:        row.TargetAccountID,
		AuthorizationID:        row.AuthorizationID,
		Status:                 row.Status,
		LeaseToken:             row.LeaseToken,
		SourceVersion:          row.SourceVersion,
		StateSeq:               row.StateSeq,
		AuthorizationExpiresAt: row.AuthorizationExpiresAt.Time,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
		RevokedAt:              revokedAt,
		CancelPending:          row.CancelPending,
		Data:                   row.Data,
	}, nil
}
