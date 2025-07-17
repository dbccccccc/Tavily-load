package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/dbccccccc/tavily-load/internal/database"
)

type APIKey struct {
	ID               int64      `db:"id"`
	KeyValue         string     `db:"key_value"`
	Name             string     `db:"name"`
	Description      string     `db:"description"`
	IsActive         bool       `db:"is_active"`
	IsBlacklisted    bool       `db:"is_blacklisted"`
	BlacklistedUntil *time.Time `db:"blacklisted_until"`
	BlacklistReason  string     `db:"blacklist_reason"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

type KeyUsageStats struct {
	ID            int64      `db:"id"`
	KeyID         int64      `db:"key_id"`
	RequestsCount int64      `db:"requests_count"`
	ErrorsCount   int64      `db:"errors_count"`
	LastUsedAt    *time.Time `db:"last_used_at"`
	LastErrorAt   *time.Time `db:"last_error_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

type BlacklistHistory struct {
	ID               int64      `db:"id"`
	KeyID            int64      `db:"key_id"`
	BlacklistedAt    time.Time  `db:"blacklisted_at"`
	BlacklistedUntil *time.Time `db:"blacklisted_until"`
	Reason           string     `db:"reason"`
	IsPermanent      bool       `db:"is_permanent"`
}

type KeyRepository struct {
	db *database.DB
}

func NewKeyRepository(db *database.DB) *KeyRepository {
	return &KeyRepository{db: db}
}

func (r *KeyRepository) CreateKey(ctx context.Context, keyValue, name, description string) (*APIKey, error) {
	query := `
		INSERT INTO api_keys (key_value, name, description, is_active, is_blacklisted)
		VALUES (?, ?, ?, true, false)
	`

	result, err := r.db.ExecContext(ctx, query, keyValue, name, description)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetKeyByID(ctx, id)
}

func (r *KeyRepository) GetKeyByID(ctx context.Context, id int64) (*APIKey, error) {
	query := `
		SELECT id, key_value, name, description, is_active, is_blacklisted, 
		       blacklisted_until, blacklist_reason, created_at, updated_at
		FROM api_keys WHERE id = ?
	`

	var key APIKey
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID, &key.KeyValue, &key.Name, &key.Description, &key.IsActive,
		&key.IsBlacklisted, &key.BlacklistedUntil, &key.BlacklistReason,
		&key.CreatedAt, &key.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (r *KeyRepository) GetKeyByValue(ctx context.Context, keyValue string) (*APIKey, error) {
	query := `
		SELECT id, key_value, name, description, is_active, is_blacklisted, 
		       blacklisted_until, blacklist_reason, created_at, updated_at
		FROM api_keys WHERE key_value = ?
	`

	var key APIKey
	err := r.db.QueryRowContext(ctx, query, keyValue).Scan(
		&key.ID, &key.KeyValue, &key.Name, &key.Description, &key.IsActive,
		&key.IsBlacklisted, &key.BlacklistedUntil, &key.BlacklistReason,
		&key.CreatedAt, &key.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (r *KeyRepository) GetAllActiveKeys(ctx context.Context) ([]*APIKey, error) {
	query := `
		SELECT id, key_value, name, description, is_active, is_blacklisted, 
		       blacklisted_until, blacklist_reason, created_at, updated_at
		FROM api_keys 
		WHERE is_active = true AND (is_blacklisted = false OR 
		      (blacklisted_until IS NOT NULL AND blacklisted_until < NOW()))
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(
			&key.ID, &key.KeyValue, &key.Name, &key.Description, &key.IsActive,
			&key.IsBlacklisted, &key.BlacklistedUntil, &key.BlacklistReason,
			&key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}

	return keys, rows.Err()
}

func (r *KeyRepository) BlacklistKey(ctx context.Context, keyValue, reason string, permanent bool, until *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get key ID
	var keyID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM api_keys WHERE key_value = ?", keyValue).Scan(&keyID)
	if err != nil {
		return err
	}

	// Update key status
	updateQuery := `
		UPDATE api_keys 
		SET is_blacklisted = true, blacklisted_until = ?, blacklist_reason = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err = tx.ExecContext(ctx, updateQuery, until, reason, keyID)
	if err != nil {
		return err
	}

	// Add to blacklist history
	historyQuery := `
		INSERT INTO key_blacklist_history (key_id, blacklisted_until, reason, is_permanent)
		VALUES (?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, historyQuery, keyID, until, reason, permanent)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *KeyRepository) UnblacklistKey(ctx context.Context, keyValue string) error {
	query := `
		UPDATE api_keys 
		SET is_blacklisted = false, blacklisted_until = NULL, blacklist_reason = '', updated_at = NOW()
		WHERE key_value = ?
	`
	_, err := r.db.ExecContext(ctx, query, keyValue)
	return err
}

func (r *KeyRepository) UpdateKeyUsage(ctx context.Context, keyValue string, requestsIncrement, errorsIncrement int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get key ID
	var keyID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM api_keys WHERE key_value = ?", keyValue).Scan(&keyID)
	if err != nil {
		return err
	}

	// Insert or update usage stats
	query := `
		INSERT INTO key_usage_stats (key_id, requests_count, errors_count, last_used_at, last_error_at)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		requests_count = requests_count + VALUES(requests_count),
		errors_count = errors_count + VALUES(errors_count),
		last_used_at = CASE WHEN VALUES(requests_count) > 0 THEN VALUES(last_used_at) ELSE last_used_at END,
		last_error_at = CASE WHEN VALUES(errors_count) > 0 THEN VALUES(last_error_at) ELSE last_error_at END,
		updated_at = NOW()
	`

	now := time.Now()
	var lastUsed, lastError *time.Time
	if requestsIncrement > 0 {
		lastUsed = &now
	}
	if errorsIncrement > 0 {
		lastError = &now
	}

	_, err = tx.ExecContext(ctx, query, keyID, requestsIncrement, errorsIncrement, lastUsed, lastError)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *KeyRepository) GetKeyStats(ctx context.Context, keyValue string) (*KeyUsageStats, error) {
	query := `
		SELECT s.id, s.key_id, s.requests_count, s.errors_count, s.last_used_at, s.last_error_at, s.created_at, s.updated_at
		FROM key_usage_stats s
		JOIN api_keys k ON s.key_id = k.id
		WHERE k.key_value = ?
	`

	var stats KeyUsageStats
	err := r.db.QueryRowContext(ctx, query, keyValue).Scan(
		&stats.ID, &stats.KeyID, &stats.RequestsCount, &stats.ErrorsCount,
		&stats.LastUsedAt, &stats.LastErrorAt, &stats.CreatedAt, &stats.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return zero stats if no record exists
		return &KeyUsageStats{RequestsCount: 0, ErrorsCount: 0}, nil
	}

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *KeyRepository) GetBlacklistHistory(ctx context.Context, keyValue string) ([]*BlacklistHistory, error) {
	query := `
		SELECT h.id, h.key_id, h.blacklisted_at, h.blacklisted_until, h.reason, h.is_permanent
		FROM key_blacklist_history h
		JOIN api_keys k ON h.key_id = k.id
		WHERE k.key_value = ?
		ORDER BY h.blacklisted_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, keyValue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*BlacklistHistory
	for rows.Next() {
		var h BlacklistHistory
		err := rows.Scan(&h.ID, &h.KeyID, &h.BlacklistedAt, &h.BlacklistedUntil, &h.Reason, &h.IsPermanent)
		if err != nil {
			return nil, err
		}
		history = append(history, &h)
	}

	return history, rows.Err()
}

func (r *KeyRepository) DeleteKey(ctx context.Context, keyValue string) error {
	query := "DELETE FROM api_keys WHERE key_value = ?"
	_, err := r.db.ExecContext(ctx, query, keyValue)
	return err
}

func (r *KeyRepository) GetAllKeys(ctx context.Context) ([]*APIKey, error) {
	query := `
		SELECT id, key_value, name, description, is_active, is_blacklisted, 
		       blacklisted_until, blacklist_reason, created_at, updated_at
		FROM api_keys
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(
			&key.ID, &key.KeyValue, &key.Name, &key.Description, &key.IsActive,
			&key.IsBlacklisted, &key.BlacklistedUntil, &key.BlacklistReason,
			&key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}

	return keys, rows.Err()
}
