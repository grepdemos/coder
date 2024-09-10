package keyrotate

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"time"

	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtime"
	"github.com/coder/quartz"
)

const (
	WorkspaceAppsTokenDuration = time.Minute
	OIDCConvertTokenDuration   = time.Minute * 5
	PeerReconnectTokenDuration = time.Hour * 24
)

type KeyRotator struct {
	DB           database.Store
	KeyDuration  time.Duration
	Clock        quartz.Clock
	Logger       slog.Logger
	ScanInterval time.Duration
}

func (kr *KeyRotator) Start(ctx context.Context) {
	ticker := kr.Clock.NewTicker(kr.ScanInterval)
	defer ticker.Stop()

	for {
		err := kr.rotateKeys(ctx)
		if err != nil {
			kr.Logger.Error("rotate keys", slog.Any("error", err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// rotateKeys checks for keys nearing expiration and rotates them if necessary.
func (kr *KeyRotator) rotateKeys(ctx context.Context) error {
	return database.ReadModifyUpdate(kr.DB, func(tx database.Store) error {
		keys, err := tx.GetKeys(ctx)
		if err != nil {
			return xerrors.Errorf("get keys: %w", err)
		}

		now := dbtime.Time(kr.Clock.Now())
		for _, key := range keys {
			switch {
			case shouldDeleteKey(key, now):
				err := tx.DeleteKey(ctx, database.DeleteKeyParams{
					Feature:  key.Feature,
					Sequence: key.Sequence,
				})
				if err != nil {
					return xerrors.Errorf("delete key: %w", err)
				}
			case shouldRotateKey(key, kr.KeyDuration, now):
				err := kr.rotateKey(ctx, tx, key)
				if err != nil {
					return xerrors.Errorf("rotate key: %w", err)
				}
			default:
				continue
			}
		}
		return nil
	})
}

func (kr *KeyRotator) rotateKey(ctx context.Context, tx database.Store, key database.Key) error {
	newStartsAt := key.ExpiresAt(kr.KeyDuration)

	secret, err := generateNewSecret(key.Feature)
	if err != nil {
		return xerrors.Errorf("generate new secret: %w", err)
	}

	// Insert new key
	err = tx.InsertKey(ctx, database.InsertKeyParams{
		Feature:  key.Feature,
		Sequence: key.Sequence + 1,
		Secret: sql.NullString{
			String: secret,
			Valid:  true,
		},
		StartsAt: newStartsAt,
	})
	if err != nil {
		return xerrors.Errorf("inserting new key: %w", err)
	}

	// Update old key's deletes_at
	maxTokenLength := tokenLength(key.Feature)
	deletesAt := newStartsAt.Add(time.Hour).Add(maxTokenLength)

	err = tx.UpdateKeyDeletesAt(ctx, database.UpdateKeyDeletesAtParams{
		Feature:  key.Feature,
		Sequence: key.Sequence,
		DeletesAt: sql.NullTime{
			Time:  deletesAt,
			Valid: true,
		},
	})
	if err != nil {
		return xerrors.Errorf("update old key's deletes_at: %w", err)
	}

	return nil
}

func generateNewSecret(feature string) (string, error) {
	switch feature {
	case "workspace_apps":
		return generateKey(96)
	case "oidc_convert":
		return generateKey(32)
	case "peer_reconnect":
		return generateKey(64)
	}
	return "", xerrors.Errorf("unknown feature: %s", feature)
}

func generateKey(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", xerrors.Errorf("rand read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func tokenLength(feature string) time.Duration {
	switch feature {
	case "workspace_apps":
		return WorkspaceAppsTokenDuration
	case "oidc_convert":
		return OIDCConvertTokenDuration
	case "peer_reconnect":
		return PeerReconnectTokenDuration
	default:
		return 0
	}
}

func shouldDeleteKey(key database.Key, now time.Time) bool {
	return key.DeletesAt.Valid && key.DeletesAt.Time.After(now)
}

func shouldRotateKey(key database.Key, keyDuration time.Duration, now time.Time) bool {
	expirationTime := key.ExpiresAt(keyDuration)
	return now.Add(time.Hour).After(expirationTime)
}
