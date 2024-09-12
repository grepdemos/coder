package keyrotate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtestutil"
	"github.com/coder/coder/v2/testutil"
	"github.com/coder/quartz"
)

func TestKeyRotator(t *testing.T) {
	t.Parallel()

	t.Run("NoExistingKeys", func(t *testing.T) {
		t.Parallel()

		var (
			db, _     = dbtestutil.NewDB(t)
			clock     = quartz.NewMock(t)
			logger    = slogtest.Make(t, nil).Leveled(slog.LevelDebug)
			ctx       = testutil.Context(t, testutil.WaitShort)
			resultsCh = make(chan []database.CryptoKey, 1)
		)

		kr := &KeyRotator{
			DB:           db,
			KeyDuration:  0,
			Clock:        clock,
			Logger:       logger,
			ScanInterval: 0,
			ResultsCh:    resultsCh,
		}

		go kr.Start(ctx)

		keys := testutil.RequireRecvCtx(ctx, t, resultsCh)
		require.Len(t, keys, len(database.AllCryptoKeyFeatureValues()))
	})

	t.Run("RotatesKeysNearExpiration", func(t *testing.T) {
		t.Parallel()

		var (
			db, _        = dbtestutil.NewDB(t)
			clock        = quartz.NewMock(t)
			keyDuration  = time.Hour * 24 * 7
			logger       = slogtest.Make(t, nil).Leveled(slog.LevelDebug)
			ctx          = testutil.Context(t, testutil.WaitShort)
			resultsCh    = make(chan []database.CryptoKey, 1)
			scanInterval = time.Minute * 10
		)

		kr := &KeyRotator{
			DB:           db,
			KeyDuration:  keyDuration,
			Clock:        clock,
			Logger:       logger,
			ScanInterval: scanInterval,
			ResultsCh:    resultsCh,
		}
		keys, err := kr.rotateKeys(ctx)
		require.NoError(t, err)
		require.Len(t, keys, len(database.AllCryptoKeyFeatureValues()))

		clock.Advance(keyDuration - time.Minute*59)
		keys, err = kr.rotateKeys(ctx)
		require.NoError(t, err)
		require.Len(t, keys, 2*len(database.AllCryptoKeyFeatureValues()))
	})

	t.Run("DoesNotRotateValidKeys", func(t *testing.T) {
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
