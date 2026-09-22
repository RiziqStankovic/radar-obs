package qan

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const postgresDigestQuery = `SELECT queryid, dbid, COALESCE(query, ''), COALESCE(calls, 0), COALESCE(total_exec_time, 0), COALESCE(min_exec_time, 0), COALESCE(max_exec_time, 0), COALESCE(mean_exec_time, 0), COALESCE(rows, 0), COALESCE(shared_blks_hit, 0), COALESCE(shared_blks_read, 0) FROM pg_stat_statements`

type PostgresConfig struct {
	DSN             string
	ServiceName     string
	AgentID         string
	Interval        time.Duration
	IncludeExamples bool
}

type PostgresCollector struct {
	config PostgresConfig
	open   func(string, string) (*sql.DB, error)
	db     *sql.DB
	mu     sync.Mutex
	prev   map[postgresKey]postgresSnapshot
}

type postgresKey struct {
	queryID int64
	dbID    uint32
}

type postgresSnapshot struct {
	calls           float64
	totalExecTimeMs float64
	rows            float64
	sharedBlksHit   float64
	sharedBlksRead  float64
}

type postgresStatement struct {
	queryID         int64
	dbID            uint32
	query           string
	calls           float64
	totalExecTimeMs float64
	minExecTimeMs   float64
	maxExecTimeMs   float64
	meanExecTimeMs  float64
	rows            float64
	sharedBlksHit   float64
	sharedBlksRead  float64
}

func NewPostgresCollector(config PostgresConfig) (*PostgresCollector, error) {
	if config.DSN == "" {
		return nil, fmt.Errorf("PostgreSQL QAN DSN is required")
	}
	if config.Interval <= 0 {
		config.Interval = 60 * time.Second
	}
	if config.ServiceName == "" {
		config.ServiceName = "postgresql"
	}
	if config.AgentID == "" {
		config.AgentID = "radar-agent"
	}
	return &PostgresCollector{config: config, open: sql.Open, prev: make(map[postgresKey]postgresSnapshot)}, nil
}

func (c *PostgresCollector) Interval() time.Duration {
	return c.config.Interval
}

func (c *PostgresCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db != nil {
		return nil
	}
	db, err := c.open("pgx", c.config.DSN)
	if err != nil {
		return fmt.Errorf("open PostgreSQL QAN database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping PostgreSQL QAN database: %w", err)
	}
	c.db = db
	return nil
}

func (c *PostgresCollector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}

func (c *PostgresCollector) Collect(ctx context.Context, periodStart time.Time) (CollectRequest, error) {
	c.mu.Lock()
	db := c.db
	c.mu.Unlock()
	if db == nil {
		return CollectRequest{}, fmt.Errorf("PostgreSQL QAN collector is not started")
	}
	rows, err := db.QueryContext(ctx, postgresDigestQuery)
	if err != nil {
		return CollectRequest{}, fmt.Errorf("query pg_stat_statements: %w", err)
	}
	defer rows.Close()

	request := CollectRequest{}
	current := make(map[postgresKey]postgresSnapshot)
	for rows.Next() {
		var statement postgresStatement
		if err := rows.Scan(&statement.queryID, &statement.dbID, &statement.query, &statement.calls, &statement.totalExecTimeMs, &statement.minExecTimeMs, &statement.maxExecTimeMs, &statement.meanExecTimeMs, &statement.rows, &statement.sharedBlksHit, &statement.sharedBlksRead); err != nil {
			return CollectRequest{}, fmt.Errorf("scan pg_stat_statements row: %w", err)
		}
		key := postgresKey{queryID: statement.queryID, dbID: statement.dbID}
		snapshot := postgresSnapshot{calls: statement.calls, totalExecTimeMs: statement.totalExecTimeMs, rows: statement.rows, sharedBlksHit: statement.sharedBlksHit, sharedBlksRead: statement.sharedBlksRead}
		current[key] = snapshot
		previous, ok := c.previous(key)
		if !ok {
			continue
		}
		calls := delta(snapshot.calls, previous.calls)
		if calls <= 0 {
			continue
		}
		request.Buckets = append(request.Buckets, Bucket{
			QueryID:             strconv.FormatInt(statement.queryID, 10),
			Fingerprint:         statement.query,
			ServiceName:         c.config.ServiceName,
			ServiceType:         "postgresql",
			AgentID:             c.config.AgentID,
			AgentType:           "postgresql-pg-stat-statements",
			PeriodStartUnixSecs: uint32(periodStart.Unix()),
			PeriodLengthSecs:    uint32(c.config.Interval.Seconds()),
			Example:             example(c.config.IncludeExamples, statement.query),
			NumQueries:          calls,
			Metrics: Metrics{
				QueryTimeCnt: calls,
				QueryTimeSum: delta(snapshot.totalExecTimeMs, previous.totalExecTimeMs) / 1000,
				QueryTimeMin: statement.minExecTimeMs / 1000,
				QueryTimeMax: statement.maxExecTimeMs / 1000,
				RowsSentSum:  delta(snapshot.rows, previous.rows),
			},
			ExtraMetrics: map[string]float64{
				"mean_exec_time":       statement.meanExecTimeMs / 1000,
				"shared_blks_hit_sum":  delta(snapshot.sharedBlksHit, previous.sharedBlksHit),
				"shared_blks_read_sum": delta(snapshot.sharedBlksRead, previous.sharedBlksRead),
			},
		})
	}
	if err := rows.Err(); err != nil {
		return CollectRequest{}, fmt.Errorf("iterate pg_stat_statements rows: %w", err)
	}
	c.mu.Lock()
	c.prev = current
	c.mu.Unlock()
	return request, nil
}

func (c *PostgresCollector) previous(key postgresKey) (postgresSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous, ok := c.prev[key]
	return previous, ok
}

func delta(current, previous float64) float64 {
	if current < previous {
		return current
	}
	return current - previous
}

func example(enabled bool, query string) string {
	if !enabled {
		return ""
	}
	return query
}
