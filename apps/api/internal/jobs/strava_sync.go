package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"golang.org/x/oauth2"

	pkgcrypto "github.com/swastikpatel7/cadence/pkg/crypto"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
	"github.com/swastikpatel7/cadence/pkg/strava"
)

const (
	providerStrava      = "strava"
	progressFlushAt     = 5 // persist sync_progress every N activities
	pageSize            = 200
	maxConsecutivePages = 50 // safety stop in case the loop never hits an empty page
)

// StravaSyncWorker is the River worker for strava.SyncJobArgs. One job
// per user-initiated manual sync.
//
// Flow per Work() invocation:
//  1. Load connected_accounts row, decrypt tokens.
//  2. Build a RotatingTokenSource so refreshed tokens get re-encrypted
//     and persisted (Strava rotates refresh tokens on every refresh).
//  3. Walk activities oldest-first using a (after, before) cursor:
//     fixed lower bound on the original window, upper bound advances
//     down toward it as we process pages.
//  4. For each activity: GET detail + GET streams, upsert in a tx.
//  5. On 429: river.JobSnooze for the parsed Retry-After window. The
//     persisted sync_progress lets us resume cleanly.
//  6. On 401 (post-refresh): mark the row failed and stop.
type StravaSyncWorker struct {
	river.WorkerDefaults[strava.SyncJobArgs]
	db      *pgxpool.Pool
	queries *dbgen.Queries
	cipher  *pkgcrypto.TokenCipher
	oauth   *oauth2.Config
}

// NewStravaSyncWorker constructs the worker with its dependencies.
func NewStravaSyncWorker(
	db *pgxpool.Pool,
	queries *dbgen.Queries,
	cipher *pkgcrypto.TokenCipher,
	oauthCfg *oauth2.Config,
) *StravaSyncWorker {
	return &StravaSyncWorker{
		db:      db,
		queries: queries,
		cipher:  cipher,
		oauth:   oauthCfg,
	}
}

// Work runs one manual-sync iteration. Returns river.JobSnooze when
// rate-limited so River will requeue with the appropriate delay.
func (w *StravaSyncWorker) Work(ctx context.Context, j *river.Job[strava.SyncJobArgs]) error {
	log := pkglogger.FromContext(ctx).With("user_id", j.Args.UserID, "job_id", j.ID)

	conn, err := w.queries.GetConnectedAccount(ctx, dbgen.GetConnectedAccountParams{
		UserID:   j.Args.UserID,
		Provider: providerStrava,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn("strava sync: no connection")
			return nil // nothing to do; don't retry
		}
		return fmt.Errorf("load connection: %w", err)
	}
	if conn.LastError != nil && *conn.LastError != "" {
		log.Warn("strava sync: connection in error state", "last_error", *conn.LastError)
		return nil
	}

	accessTok, err := w.cipher.Decrypt(conn.AccessTokenEnc)
	if err != nil {
		return fmt.Errorf("decrypt access token: %w", err)
	}
	refreshTok, err := w.cipher.Decrypt(conn.RefreshTokenEnc)
	if err != nil {
		return fmt.Errorf("decrypt refresh token: %w", err)
	}

	initial := &oauth2.Token{
		AccessToken:  accessTok,
		RefreshToken: refreshTok,
		TokenType:    "Bearer",
		Expiry:       conn.ExpiresAt.Time,
	}
	connID := conn.ID
	rts := strava.NewRotatingTokenSource(
		ctx,
		w.oauth.TokenSource(ctx, initial),
		initial,
		func(ctx context.Context, t *oauth2.Token) error {
			a, err := w.cipher.Encrypt(t.AccessToken)
			if err != nil {
				return err
			}
			r, err := w.cipher.Encrypt(t.RefreshToken)
			if err != nil {
				return err
			}
			return w.queries.UpdateConnectedAccountTokens(ctx, dbgen.UpdateConnectedAccountTokensParams{
				ID:              connID,
				AccessTokenEnc:  a,
				RefreshTokenEnc: r,
				ExpiresAt:       pgtype.Timestamptz{Time: t.Expiry, Valid: true},
			})
		},
	)
	client := strava.NewClient(ctx, rts)

	var progress strava.SyncProgress
	if len(conn.SyncProgress) > 0 {
		_ = json.Unmarshal(conn.SyncProgress, &progress)
	}
	if progress.AfterTs == 0 {
		progress.AfterTs = j.Args.AfterTs
	}

	for iter := 0; iter < maxConsecutivePages; iter++ {
		summaries, err := client.ListAthleteActivities(ctx, progress.AfterTs, progress.BeforeTs, pageSize, 1)
		if err != nil {
			return w.handleStravaError(ctx, log, conn.ID, &progress, err)
		}
		if len(summaries) == 0 {
			break
		}

		var oldestStart time.Time
		for _, raw := range summaries {
			var sa strava.SummaryActivity
			if jerr := json.Unmarshal(raw, &sa); jerr != nil {
				log.Warn("decode summary failed", "err", jerr)
				continue
			}
			startTime, terr := time.Parse(time.RFC3339, sa.StartDate)
			if terr != nil {
				log.Warn("parse start_date failed", "err", terr, "id", sa.ID)
				continue
			}

			detail, err := client.GetActivityDetail(ctx, sa.ID)
			if err != nil {
				return w.handleStravaError(ctx, log, conn.ID, &progress, err)
			}
			streams, err := client.GetActivityStreams(ctx, sa.ID, nil)
			if err != nil {
				return w.handleStravaError(ctx, log, conn.ID, &progress, err)
			}

			if err := w.upsertActivity(ctx, j.Args.UserID, sa, startTime, detail, streams); err != nil {
				return fmt.Errorf("upsert activity %d: %w", sa.ID, err)
			}

			progress.Processed++
			if oldestStart.IsZero() || startTime.Before(oldestStart) {
				oldestStart = startTime
			}
			if progress.Processed%progressFlushAt == 0 {
				w.persistProgress(ctx, log, conn.ID, &progress)
			}
		}

		if !oldestStart.IsZero() {
			progress.BeforeTs = oldestStart.Unix()
		}
		w.persistProgress(ctx, log, conn.ID, &progress)

		if len(summaries) < pageSize {
			break
		}
	}

	if err := w.queries.ClearSyncStartedSuccess(ctx, conn.ID); err != nil {
		return fmt.Errorf("clear sync state: %w", err)
	}
	log.Info("strava sync complete", "processed", progress.Processed)
	return nil
}

// handleStravaError translates Strava client errors into the right
// outcome: 429 → JobSnooze; 401 → mark connection failed; other → retry
// per River's default policy.
func (w *StravaSyncWorker) handleStravaError(
	ctx context.Context,
	log interface{ Warn(string, ...any) },
	connID uuid.UUID,
	progress *strava.SyncProgress,
	err error,
) error {
	if rl, ok := strava.IsRateLimited(err); ok {
		w.persistProgress(ctx, log, connID, progress)
		log.Warn("strava sync: rate limited", "retry_after", rl.RetryAfter)
		return river.JobSnooze(rl.RetryAfter)
	}
	var authErr *strava.AuthError
	if errors.As(err, &authErr) {
		_ = w.queries.ClearSyncStartedFailure(ctx, dbgen.ClearSyncStartedFailureParams{
			ID:        connID,
			LastError: ptr("Strava auth failed; please reconnect"),
		})
		log.Warn("strava sync: auth failed", "status", authErr.StatusCode)
		// nil so River treats this as terminal — don't retry.
		return nil
	}
	return err
}

func (w *StravaSyncWorker) persistProgress(
	ctx context.Context,
	log interface{ Warn(string, ...any) },
	connID uuid.UUID,
	progress *strava.SyncProgress,
) {
	pbytes, err := json.Marshal(progress)
	if err != nil {
		log.Warn("marshal progress failed", "err", err)
		return
	}
	if err := w.queries.SetSyncProgress(ctx, dbgen.SetSyncProgressParams{
		ID:           connID,
		SyncProgress: pbytes,
	}); err != nil {
		log.Warn("persist progress failed", "err", err)
	}
}

func (w *StravaSyncWorker) upsertActivity(
	ctx context.Context,
	userID uuid.UUID,
	sa strava.SummaryActivity,
	startTime time.Time,
	detail, streams []byte,
) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := w.queries.WithTx(tx)

	sportType := sa.SportType
	if sportType == "" {
		sportType = sa.Type
	}
	if sportType == "" {
		sportType = "Workout"
	}

	row, err := q.UpsertActivity(ctx, dbgen.UpsertActivityParams{
		UserID:          userID,
		Source:          providerStrava,
		SourceID:        fmt.Sprintf("%d", sa.ID),
		SportType:       sportType,
		Name:            sa.Name,
		StartTime:       pgtype.Timestamptz{Time: startTime, Valid: true},
		DurationSeconds: int32(sa.MovingTime),
		DistanceMeters:  numericFromFloat(sa.Distance),
		ElevationGainM:  numericFromFloat(sa.TotalElevationGain),
		AvgHeartRate:    intPtrFromFloat(sa.AverageHeartrate),
		MaxHeartRate:    intPtrFromFloat(sa.MaxHeartrate),
		Calories:        intPtrFromFloat(sa.Calories),
		Raw:             detail,
	})
	if err != nil {
		return fmt.Errorf("upsert activity: %w", err)
	}

	if len(streams) > 0 {
		if err := q.UpsertActivityStreams(ctx, dbgen.UpsertActivityStreamsParams{
			ActivityID: row.ID,
			Streams:    streams,
		}); err != nil {
			return fmt.Errorf("upsert streams: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func numericFromFloat(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{Valid: false}
	}
	// Two-decimal scale matches the numeric(*, 2) columns we use.
	return pgtype.Numeric{
		Int:   big.NewInt(int64(f * 100)),
		Exp:   -2,
		Valid: true,
	}
}

func intPtrFromFloat(f float64) *int32 {
	if f <= 0 {
		return nil
	}
	v := int32(f)
	return &v
}

func ptr[T any](v T) *T { return &v }
