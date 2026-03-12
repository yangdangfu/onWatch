package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/onllm-dev/onwatch/v2/internal/api"
)

// KimiResetCycle represents a Kimi quota reset cycle
type KimiResetCycle struct {
	ID         int64
	QuotaType  string
	CycleStart time.Time
	CycleEnd   *time.Time
	NextReset  *time.Time
	PeakValue  int64
	TotalDelta int64
}

// InsertKimiSnapshot inserts a Kimi quota snapshot
func (s *Store) InsertKimiSnapshot(snapshot *api.KimiSnapshot) (int64, error) {
	var tokensNextReset interface{}
	if snapshot.TokensNextResetTime != nil {
		tokensNextReset = snapshot.TokensNextResetTime.Format(time.RFC3339Nano)
	} else {
		tokensNextReset = nil
	}

	result, err := s.db.Exec(
		`INSERT INTO kimi_snapshots
		(provider, captured_at, time_limit, time_unit, time_number, time_usage,
		 time_current_value, time_remaining, time_percentage, time_usage_details,
		 tokens_limit, tokens_unit, tokens_number, tokens_usage,
		 tokens_current_value, tokens_remaining, tokens_percentage, tokens_next_reset)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"kimi",
		snapshot.CapturedAt.Format(time.RFC3339Nano),
		snapshot.TimeLimit, snapshot.TimeUnit, snapshot.TimeNumber,
		snapshot.TimeUsage, snapshot.TimeCurrentValue, snapshot.TimeRemaining, snapshot.TimePercentage,
		snapshot.TimeUsageDetails,
		snapshot.TokensLimit, snapshot.TokensUnit, snapshot.TokensNumber,
		snapshot.TokensUsage, snapshot.TokensCurrentValue, snapshot.TokensRemaining, snapshot.TokensPercentage,
		tokensNextReset,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert kimi snapshot: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// QueryLatestKimi returns the most recent Kimi snapshot
func (s *Store) QueryLatestKimi() (*api.KimiSnapshot, error) {
	var snapshot api.KimiSnapshot
	var capturedAt string
	var tokensNextReset sql.NullString

	err := s.db.QueryRow(
		`SELECT id, captured_at, time_limit, time_unit, time_number, time_usage,
		 time_current_value, time_remaining, time_percentage, time_usage_details,
		 tokens_limit, tokens_unit, tokens_number, tokens_usage,
		 tokens_current_value, tokens_remaining, tokens_percentage, tokens_next_reset
		FROM kimi_snapshots ORDER BY captured_at DESC LIMIT 1`,
	).Scan(
		&snapshot.ID, &capturedAt, &snapshot.TimeLimit, &snapshot.TimeUnit, &snapshot.TimeNumber,
		&snapshot.TimeUsage, &snapshot.TimeCurrentValue, &snapshot.TimeRemaining, &snapshot.TimePercentage,
		&snapshot.TimeUsageDetails,
		&snapshot.TokensLimit, &snapshot.TokensUnit, &snapshot.TokensNumber,
		&snapshot.TokensUsage, &snapshot.TokensCurrentValue, &snapshot.TokensRemaining, &snapshot.TokensPercentage,
		&tokensNextReset,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest kimi: %w", err)
	}

	snapshot.CapturedAt, _ = time.Parse(time.RFC3339Nano, capturedAt)
	if tokensNextReset.Valid && tokensNextReset.String != "" {
		t, _ := time.Parse(time.RFC3339Nano, tokensNextReset.String)
		snapshot.TokensNextResetTime = &t
	}

	return &snapshot, nil
}

// QueryKimiRange returns Kimi snapshots within a time range with optional limit.
func (s *Store) QueryKimiRange(start, end time.Time, limit ...int) ([]*api.KimiSnapshot, error) {
	query := `SELECT id, captured_at, time_limit, time_unit, time_number, time_usage,
		 time_current_value, time_remaining, time_percentage, time_usage_details,
		 tokens_limit, tokens_unit, tokens_number, tokens_usage,
		 tokens_current_value, tokens_remaining, tokens_percentage, tokens_next_reset
		FROM kimi_snapshots
		WHERE captured_at BETWEEN ? AND ?
		ORDER BY captured_at ASC`
	args := []interface{}{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)}
	if len(limit) > 0 && limit[0] > 0 {
		query = `SELECT id, captured_at, time_limit, time_unit, time_number, time_usage,
			 time_current_value, time_remaining, time_percentage, time_usage_details,
			 tokens_limit, tokens_unit, tokens_number, tokens_usage,
			 tokens_current_value, tokens_remaining, tokens_percentage, tokens_next_reset
			FROM (
				SELECT id, captured_at, time_limit, time_unit, time_number, time_usage,
					 time_current_value, time_remaining, time_percentage, time_usage_details,
					 tokens_limit, tokens_unit, tokens_number, tokens_usage,
					 tokens_current_value, tokens_remaining, tokens_percentage, tokens_next_reset
				FROM kimi_snapshots
				WHERE captured_at BETWEEN ? AND ?
				ORDER BY captured_at DESC
				LIMIT ?
			) recent
			ORDER BY captured_at ASC`
		args = append(args, limit[0])
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query kimi range: %w", err)
	}
	defer rows.Close()

	var snapshots []*api.KimiSnapshot
	for rows.Next() {
		var snapshot api.KimiSnapshot
		var capturedAt string
		var tokensNextReset sql.NullString

		err := rows.Scan(
			&snapshot.ID, &capturedAt, &snapshot.TimeLimit, &snapshot.TimeUnit, &snapshot.TimeNumber,
			&snapshot.TimeUsage, &snapshot.TimeCurrentValue, &snapshot.TimeRemaining, &snapshot.TimePercentage,
			&snapshot.TimeUsageDetails,
			&snapshot.TokensLimit, &snapshot.TokensUnit, &snapshot.TokensNumber,
			&snapshot.TokensUsage, &snapshot.TokensCurrentValue, &snapshot.TokensRemaining, &snapshot.TokensPercentage,
			&tokensNextReset,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kimi snapshot: %w", err)
		}

		snapshot.CapturedAt, _ = time.Parse(time.RFC3339Nano, capturedAt)
		if tokensNextReset.Valid && tokensNextReset.String != "" {
			t, _ := time.Parse(time.RFC3339Nano, tokensNextReset.String)
			snapshot.TokensNextResetTime = &t
		}

		snapshots = append(snapshots, &snapshot)
	}

	return snapshots, rows.Err()
}

// CreateKimiCycle creates a new Kimi reset cycle
func (s *Store) CreateKimiCycle(quotaType string, cycleStart time.Time, nextReset *time.Time) (int64, error) {
	var nextResetValue interface{}
	if nextReset != nil {
		nextResetValue = nextReset.Format(time.RFC3339Nano)
	} else {
		nextResetValue = nil
	}

	result, err := s.db.Exec(
		`INSERT INTO kimi_reset_cycles (quota_type, cycle_start, next_reset) VALUES (?, ?, ?)`,
		quotaType, cycleStart.Format(time.RFC3339Nano), nextResetValue,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create kimi cycle: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get cycle ID: %w", err)
	}

	return id, nil
}

// CloseKimiCycle closes a Kimi reset cycle with final stats
func (s *Store) CloseKimiCycle(quotaType string, cycleEnd time.Time, peak, delta int64) error {
	_, err := s.db.Exec(
		`UPDATE kimi_reset_cycles SET cycle_end = ?, peak_value = ?, total_delta = ?
		WHERE quota_type = ? AND cycle_end IS NULL`,
		cycleEnd.Format(time.RFC3339Nano), peak, delta, quotaType,
	)
	if err != nil {
		return fmt.Errorf("failed to close kimi cycle: %w", err)
	}
	return nil
}

// UpdateKimiCycle updates the peak and delta for an active Kimi cycle
func (s *Store) UpdateKimiCycle(quotaType string, peak, delta int64) error {
	_, err := s.db.Exec(
		`UPDATE kimi_reset_cycles SET peak_value = ?, total_delta = ?
		WHERE quota_type = ? AND cycle_end IS NULL`,
		peak, delta, quotaType,
	)
	if err != nil {
		return fmt.Errorf("failed to update kimi cycle: %w", err)
	}
	return nil
}

// QueryActiveKimiCycle returns the active cycle for a Kimi quota type
func (s *Store) QueryActiveKimiCycle(quotaType string) (*KimiResetCycle, error) {
	var cycle KimiResetCycle
	var cycleStart string
	var cycleEnd, nextReset sql.NullString

	err := s.db.QueryRow(
		`SELECT id, quota_type, cycle_start, cycle_end, next_reset, peak_value, total_delta
		FROM kimi_reset_cycles WHERE quota_type = ? AND cycle_end IS NULL`,
		quotaType,
	).Scan(
		&cycle.ID, &cycle.QuotaType, &cycleStart, &cycleEnd, &nextReset, &cycle.PeakValue, &cycle.TotalDelta,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query active kimi cycle: %w", err)
	}

	cycle.CycleStart, _ = time.Parse(time.RFC3339Nano, cycleStart)
	if cycleEnd.Valid {
		endTime, _ := time.Parse(time.RFC3339Nano, cycleEnd.String)
		cycle.CycleEnd = &endTime
	}
	if nextReset.Valid {
		resetTime, _ := time.Parse(time.RFC3339Nano, nextReset.String)
		cycle.NextReset = &resetTime
	}

	return &cycle, nil
}

// QueryKimiCycleHistory returns completed cycles for a Kimi quota type with optional limit.
func (s *Store) QueryKimiCycleHistory(quotaType string, limit ...int) ([]*KimiResetCycle, error) {
	query := `SELECT id, quota_type, cycle_start, cycle_end, next_reset, peak_value, total_delta
		FROM kimi_reset_cycles WHERE quota_type = ? AND cycle_end IS NOT NULL ORDER BY cycle_start DESC`
	args := []interface{}{quotaType}
	if len(limit) > 0 && limit[0] > 0 {
		query += ` LIMIT ?`
		args = append(args, limit[0])
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query kimi cycles: %w", err)
	}
	defer rows.Close()

	var cycles []*KimiResetCycle
	for rows.Next() {
		var cycle KimiResetCycle
		var cycleStart, cycleEnd string
		var nextReset sql.NullString

		err := rows.Scan(
			&cycle.ID, &cycle.QuotaType, &cycleStart, &cycleEnd, &nextReset, &cycle.PeakValue, &cycle.TotalDelta,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kimi cycle: %w", err)
		}

		cycle.CycleStart, _ = time.Parse(time.RFC3339Nano, cycleStart)
		endTime, _ := time.Parse(time.RFC3339Nano, cycleEnd)
		cycle.CycleEnd = &endTime
		if nextReset.Valid {
			resetTime, _ := time.Parse(time.RFC3339Nano, nextReset.String)
			cycle.NextReset = &resetTime
		}

		cycles = append(cycles, &cycle)
	}

	return cycles, rows.Err()
}

// QueryKimiCycleOverview returns Kimi cycles for a given quota type
// with cross-quota snapshot data at the peak moment of each cycle.
// Includes the currently active cycle (if any) at the top.
func (s *Store) QueryKimiCycleOverview(groupBy string, limit int) ([]CycleOverviewRow, error) {
	if limit <= 0 {
		limit = 50
	}

	// Get active cycle first (if any)
	var allCycles []*KimiResetCycle
	activeCycle, err := s.QueryActiveKimiCycle(groupBy)
	if err != nil {
		return nil, fmt.Errorf("store.QueryKimiCycleOverview: active: %w", err)
	}
	if activeCycle != nil {
		allCycles = append(allCycles, activeCycle)
		limit-- // Reduce limit for completed cycles
	}

	// Get completed cycles
	completedCycles, err := s.QueryKimiCycleHistory(groupBy, limit)
	if err != nil {
		return nil, fmt.Errorf("store.QueryKimiCycleOverview: %w", err)
	}
	allCycles = append(allCycles, completedCycles...)

	var overviewRows []CycleOverviewRow
	for _, c := range allCycles {
		row := CycleOverviewRow{
			CycleID:    c.ID,
			QuotaType:  c.QuotaType,
			CycleStart: c.CycleStart,
			CycleEnd:   c.CycleEnd, // nil for active cycles
			PeakValue:  float64(c.PeakValue),
			TotalDelta: float64(c.TotalDelta),
		}

		var peakCol string
		switch groupBy {
		case "tokens":
			peakCol = "tokens_current_value"
		case "time":
			peakCol = "time_current_value"
		default:
			peakCol = "tokens_current_value"
		}

		// Determine the end boundary for the snapshot query
		// For active cycles (no cycle_end), use current time
		// For completed cycles, use cycle_end (exclusive)
		var endBoundary time.Time
		if c.CycleEnd != nil {
			endBoundary = *c.CycleEnd
		} else {
			endBoundary = time.Now().Add(time.Minute)
		}

		var capturedAt string
		var timeUsage, timeCurrent, tokensUsage, tokensCurrent float64
		err = s.db.QueryRow(
			fmt.Sprintf(`SELECT captured_at, time_usage, time_current_value, tokens_usage, tokens_current_value
			FROM kimi_snapshots
			WHERE captured_at >= ? AND captured_at < ?
			ORDER BY %s DESC LIMIT 1`, peakCol),
			c.CycleStart.Format(time.RFC3339Nano),
			endBoundary.Format(time.RFC3339Nano),
		).Scan(&capturedAt, &timeUsage, &timeCurrent, &tokensUsage, &tokensCurrent)

		if err == sql.ErrNoRows {
			overviewRows = append(overviewRows, row)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("store.QueryKimiCycleOverview: peak snapshot: %w", err)
		}

		row.PeakTime, _ = time.Parse(time.RFC3339Nano, capturedAt)

		pct := func(val, lim float64) float64 {
			if lim == 0 {
				return 0
			}
			return val / lim * 100
		}
		row.CrossQuotas = []CrossQuotaEntry{
			{Name: "tokens", Value: tokensCurrent, Limit: tokensUsage, Percent: pct(tokensCurrent, tokensUsage)},
			{Name: "time", Value: timeCurrent, Limit: timeUsage, Percent: pct(timeCurrent, timeUsage)},
		}

		overviewRows = append(overviewRows, row)
	}

	return overviewRows, nil
}

// QueryKimiCyclesSince returns all Kimi cycles (completed and active) for a quota type since a given time.
func (s *Store) QueryKimiCyclesSince(quotaType string, since time.Time) ([]*KimiResetCycle, error) {
	rows, err := s.db.Query(
		`SELECT id, quota_type, cycle_start, cycle_end, next_reset, peak_value, total_delta
		FROM kimi_reset_cycles WHERE quota_type = ? AND cycle_start >= ? ORDER BY cycle_start DESC`,
		quotaType, since.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query kimi cycles since: %w", err)
	}
	defer rows.Close()

	var cycles []*KimiResetCycle
	for rows.Next() {
		var cycle KimiResetCycle
		var cycleStart string
		var cycleEnd, nextReset sql.NullString

		err := rows.Scan(
			&cycle.ID, &cycle.QuotaType, &cycleStart, &cycleEnd, &nextReset, &cycle.PeakValue, &cycle.TotalDelta,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kimi cycle: %w", err)
		}

		cycle.CycleStart, _ = time.Parse(time.RFC3339Nano, cycleStart)
		if cycleEnd.Valid {
			endTime, _ := time.Parse(time.RFC3339Nano, cycleEnd.String)
			cycle.CycleEnd = &endTime
		}
		if nextReset.Valid {
			resetTime, _ := time.Parse(time.RFC3339Nano, nextReset.String)
			cycle.NextReset = &resetTime
		}

		cycles = append(cycles, &cycle)
	}

	return cycles, rows.Err()
}
