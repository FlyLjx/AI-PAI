package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/cleanupstatus"
	"aipi-go/internal/resultdata"
)

const (
	imageDataCleanupMigrationKey = "migration.image_data_cleanup_v1"
	imageDataCleanupBatchSize    = 25
	imageDataCleanupBatchPause   = 250 * time.Millisecond
)

// CleanupLegacyGenerationImageData removes historical inline image bytes in
// small, interruptible batches. It is intended to run after HTTP startup.
func CleanupLegacyGenerationImageData(ctx context.Context, db *sql.DB, now time.Time) error {
	return cleanupLegacyGenerationImageData(ctx, db, now, imageDataCleanupBatchPause)
}

func cleanupLegacyGenerationImageData(ctx context.Context, db *sql.DB, now time.Time, batchPause time.Duration) error {
	location := appclock.ConfigureDefault()
	if now.IsZero() {
		now = time.Now()
	}
	localNow := now.In(location)
	cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)

	var marker string
	err := db.QueryRowContext(ctx, Rebind(`
		SELECT setting_value
		FROM system_settings
		WHERE setting_key = ?
	`), imageDataCleanupMigrationKey).Scan(&marker)
	if err == nil && strings.TrimSpace(marker) == "complete" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	tracker := cleanupstatus.FromContext(ctx)
	if tracker != nil {
		var total int64
		resultTextExpression := "CAST(result_json AS CHAR)"
		if CurrentDialect() == DialectPostgres {
			resultTextExpression = "result_json::text"
		}
		countQuery := `
			SELECT COUNT(*)
			FROM generation_tasks
			WHERE created_at < ?
				AND status IN ('success', 'failed', 'canceled', 'cancelled')
				AND (
					LOWER(COALESCE(reference_image_url, '')) LIKE '%data:image/%'
					OR LOWER(COALESCE(` + resultTextExpression + `, '')) LIKE '%b64_json%'
					OR LOWER(COALESCE(` + resultTextExpression + `, '')) LIKE '%base64%'
				)
		`
		if err := db.QueryRowContext(ctx, Rebind(countQuery), cutoff).Scan(&total); err != nil {
			return err
		}
		tracker.SetTotalRows(total)
	}

	lastID := ""
	resultTextExpression := "CAST(result_json AS CHAR)"
	if CurrentDialect() == DialectPostgres {
		resultTextExpression = "result_json::text"
	}
	for {
		query := `
			SELECT id, reference_image_url, result_json
			FROM generation_tasks
			WHERE created_at < ?
				AND status IN ('success', 'failed', 'canceled', 'cancelled')
				AND id > ?
				AND (
					LOWER(COALESCE(reference_image_url, '')) LIKE '%data:image/%'
					OR LOWER(COALESCE(` + resultTextExpression + `, '')) LIKE '%b64_json%'
					OR LOWER(COALESCE(` + resultTextExpression + `, '')) LIKE '%base64%'
				)
			ORDER BY id ASC
			LIMIT ?
		`
		rows, err := db.QueryContext(ctx, Rebind(query), cutoff, lastID, imageDataCleanupBatchSize)
		if err != nil {
			return err
		}
		items, readErr := scanImageDataCleanupRows(rows)
		if readErr != nil {
			return readErr
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return err
			}
			released, err := updateCleanedGenerationImageData(ctx, db, item)
			if err != nil {
				return err
			}
			if tracker != nil {
				tracker.AddDatabase(1, released)
			}
			lastID = item.id
		}
		if batchPause > 0 {
			timer := time.NewTimer(batchPause)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	_, err = db.ExecContext(ctx, NormalizeQuery(`
		INSERT INTO system_settings (setting_key, setting_value)
		VALUES (?, 'complete')
		ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)
	`), imageDataCleanupMigrationKey)
	return err
}

type imageDataCleanupRow struct {
	id             string
	referenceImage sql.NullString
	resultJSON     sql.NullString
}

func scanImageDataCleanupRows(rows *sql.Rows) ([]imageDataCleanupRow, error) {
	defer rows.Close()
	items := make([]imageDataCleanupRow, 0, imageDataCleanupBatchSize)
	for rows.Next() {
		var item imageDataCleanupRow
		if err := rows.Scan(&item.id, &item.referenceImage, &item.resultJSON); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func updateCleanedGenerationImageData(ctx context.Context, db *sql.DB, item imageDataCleanupRow) (int64, error) {
	originalBytes := 0
	cleanedBytes := 0
	var referenceValue any
	referenceChanged := false
	if item.referenceImage.Valid {
		originalBytes += len(item.referenceImage.String)
		cleaned, changed := resultdata.ReferenceURLsOnly(item.referenceImage.String)
		referenceChanged = changed
		if cleaned != nil {
			referenceValue = *cleaned
			cleanedBytes += len(*cleaned)
		}
	}
	var resultValue any
	resultChanged := false
	if item.resultJSON.Valid {
		originalBytes += len(item.resultJSON.String)
		cleaned, changed, err := resultdata.SanitizeResultJSON(item.resultJSON.String)
		if err != nil {
			return 0, err
		}
		resultChanged = changed
		if cleaned != nil {
			resultValue = *cleaned
			cleanedBytes += len(*cleaned)
		}
	}
	if !referenceChanged && !resultChanged {
		return 0, nil
	}
	if !referenceChanged {
		if item.referenceImage.Valid {
			referenceValue = item.referenceImage.String
			cleanedBytes += len(item.referenceImage.String)
		}
	}
	if !resultChanged {
		if item.resultJSON.Valid {
			resultValue = item.resultJSON.String
			cleanedBytes += len(item.resultJSON.String)
		}
	}
	_, err := db.ExecContext(ctx, Rebind(`
		UPDATE generation_tasks
		SET reference_image_url = ?, result_json = ?
		WHERE id = ?
	`), referenceValue, resultValue, item.id)
	if err != nil {
		return 0, err
	}
	released := originalBytes - cleanedBytes
	if released < 0 {
		released = 0
	}
	return int64(released), nil
}
