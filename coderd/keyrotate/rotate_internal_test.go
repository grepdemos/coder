package keyrotate

import (
	"database/sql"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbgen"
	"github.com/coder/coder/v2/coderd/database/dbtestutil"
	"github.com/coder/coder/v2/coderd/database/dbtime"
	"github.com/coder/coder/v2/testutil"
	"github.com/coder/quartz"
)

func Test_rotateKeys(t *testing.T) {
	t.Parallel()

	t.Run("RotatesKeysNearExpiration", func(t *testing.T) {
		t.Parallel()

		var (
			db, _       = dbtestutil.NewDB(t)
			clock       = quartz.NewMock(t)
			keyDuration = time.Hour * 24 * 7
			logger      = slogtest.Make(t, nil).Leveled(slog.LevelDebug)
			ctx         = testutil.Context(t, testutil.WaitShort)
			resultsCh   = make(chan []database.CryptoKey, 1)
		)

		kr := &KeyRotator{
			DB:           db,
			KeyDuration:  keyDuration,
			Clock:        clock,
			Logger:       logger,
			ScanInterval: 0,
			ResultsCh:    resultsCh,
			features: []database.CryptoKeyFeature{
				database.CryptoKeyFeatureWorkspaceApps,
			},
		}

		now := dbnow(clock)

		// Seed the database with an existing key.
		oldKey := dbgen.CryptoKey(t, db, database.CryptoKey{
			Feature:  database.CryptoKeyFeatureWorkspaceApps,
			StartsAt: now,
			Sequence: 15,
		})

		// Advance the window to just inside rotation time.
		_ = clock.Advance(keyDuration - time.Minute*59)
		keys, err := kr.rotateKeys(ctx)
		require.NoError(t, err)
		require.Len(t, keys, 2)

		now = dbnow(clock)
		expectedDeletesAt := oldKey.ExpiresAt(keyDuration).Add(WorkspaceAppsTokenDuration + time.Hour)

		// Fetch the old key, it should have an expires_at now.
		oldKey, err = db.GetCryptoKeyByFeatureAndSequence(ctx, database.GetCryptoKeyByFeatureAndSequenceParams{
			Feature:  oldKey.Feature,
			Sequence: oldKey.Sequence,
		})
		require.NoError(t, err)
		require.Equal(t, oldKey.DeletesAt.Time.UTC(), expectedDeletesAt)

		// The new key should be created and have a starts_at of the old key's expires_at.
		newKey, err := db.GetCryptoKeyByFeatureAndSequence(ctx, database.GetCryptoKeyByFeatureAndSequenceParams{
			Feature:  database.CryptoKeyFeatureWorkspaceApps,
			Sequence: oldKey.Sequence + 1,
		})
		require.NoError(t, err)
		requireKey(t, newKey, database.CryptoKeyFeatureWorkspaceApps, oldKey.ExpiresAt(keyDuration), time.Time{}, oldKey.Sequence+1)

		// Advance the clock just past the keys delete time.
		clock.Advance(oldKey.DeletesAt.Time.UTC().Sub(now) - time.Second)

		// We should have deleted the old key.
		keys, err = kr.rotateKeys(ctx)
		require.NoError(t, err)
		require.Len(t, keys, 1)

		// The old key should be "deleted".
		_, err = db.GetCryptoKeyByFeatureAndSequence(ctx, database.GetCryptoKeyByFeatureAndSequenceParams{
			Feature:  oldKey.Feature,
			Sequence: oldKey.Sequence,
		})
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("DoesNotRotateValidKeys", func(t *testing.T) {
		t.Parallel()

		var (
			db, _       = dbtestutil.NewDB(t)
			clock       = quartz.NewMock(t)
			keyDuration = time.Hour * 24 * 7
			logger      = slogtest.Make(t, nil).Leveled(slog.LevelDebug)
			ctx         = testutil.Context(t, testutil.WaitShort)
			resultsCh   = make(chan []database.CryptoKey, 1)
		)

		kr := &KeyRotator{
			DB:           db,
			KeyDuration:  keyDuration,
			Clock:        clock,
			Logger:       logger,
			ScanInterval: 0,
			ResultsCh:    resultsCh,
			features: []database.CryptoKeyFeature{
				database.CryptoKeyFeatureWorkspaceApps,
			},
		}

		now := dbnow(clock)

		// Seed the database with an existing key
		existingKey := dbgen.CryptoKey(t, db, database.CryptoKey{
			Feature:  database.CryptoKeyFeatureWorkspaceApps,
			StartsAt: now,
			Sequence: 1,
		})

		// Advance the clock by 6 days, 23 hours. Once we
		// breach the last hour we will insert a new key.
		clock.Advance(keyDuration - time.Hour)

		keys, err := kr.rotateKeys(ctx)
		require.NoError(t, err)
		require.Empty(t, keys)

		// Verify that the existing key is still the only key in the database
		dbKeys, err := db.GetCryptoKeys(ctx)
		require.NoError(t, err)
		require.Len(t, dbKeys, 1)
		requireKey(t, dbKeys[0], existingKey.Feature, existingKey.StartsAt.UTC(), existingKey.DeletesAt.Time.UTC(), existingKey.Sequence)
	})

	t.Run("DeletesExpiredKeys", func(t *testing.T) {
		t.Parallel()
		// TODO: Implement test for deleting expired keys
	})

	t.Run("HandlesMultipleKeyTypes", func(t *testing.T) {
		t.Parallel()
		// TODO: Implement test for handling multiple key types
	})

	t.Run("GracefullyHandlesErrors", func(t *testing.T) {
		t.Parallel()
		// TODO: Implement test for error handling
	})
}

func dbnow(c quartz.Clock) time.Time {
	return dbtime.Time(c.Now().UTC())
}

func requireKey(t *testing.T, key database.CryptoKey, feature database.CryptoKeyFeature, startsAt time.Time, deletesAt time.Time, sequence int32) {
	t.Helper()
	require.Equal(t, feature, key.Feature)
	require.Equal(t, startsAt, key.StartsAt.UTC())
	require.Equal(t, deletesAt, key.DeletesAt.Time.UTC())
	require.Equal(t, sequence, key.Sequence)

	secret, err := hex.DecodeString(key.Secret.String)
	require.NoError(t, err)

	switch key.Feature {
	case database.CryptoKeyFeatureOidcConvert:
		require.Len(t, secret, 32)
	case database.CryptoKeyFeatureWorkspaceApps:
		require.Len(t, secret, 96)
	case database.CryptoKeyFeaturePeerReconnect:
		require.Len(t, secret, 64)
	default:
		t.Fatalf("unknown key feature: %s", key.Feature)
	}
}
