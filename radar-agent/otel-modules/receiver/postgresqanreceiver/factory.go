package postgresqanreceiver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
)

const typeStr = "postgresqan"
const digestQuery = `SELECT queryid, dbid, COALESCE(query, ''), COALESCE(calls, 0), COALESCE(total_exec_time, 0), COALESCE(min_exec_time, 0), COALESCE(max_exec_time, 0), COALESCE(mean_exec_time, 0), COALESCE(rows, 0), COALESCE(shared_blks_hit, 0), COALESCE(shared_blks_read, 0) FROM pg_stat_statements`

type Config struct {
	DSN             string        `mapstructure:"dsn"`
	ServiceName     string        `mapstructure:"service_name"`
	AgentID         string        `mapstructure:"agent_id"`
	Interval        time.Duration `mapstructure:"interval"`
	IncludeExamples bool          `mapstructure:"include_query_examples"`
}

func createDefaultConfig() component.Config {
	return &Config{ServiceName: "postgresql", AgentID: "radar-agent", Interval: time.Minute}
}
func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(component.MustNewType(typeStr), createDefaultConfig, xreceiver.WithLogs(createLogs, component.StabilityLevelAlpha))
}

func createLogs(_ context.Context, _ receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid postgresqan config type %T", cfg)
	}
	if config.DSN == "" {
		return nil, fmt.Errorf("postgresqan dsn is required")
	}
	return newCollector(*config, next)
}

type statementKey struct {
	queryID int64
	dbID    uint32
}
type statementSnapshot struct{ calls, total, rows, hit, read float64 }
type receiverImpl struct {
	config Config
	next   consumer.Logs
	cancel context.CancelFunc
	done   chan struct{}
	db     *sql.DB
	mu     sync.Mutex
	prev   map[statementKey]statementSnapshot
}

func newCollector(config Config, next consumer.Logs) (*receiverImpl, error) {
	if config.Interval <= 0 {
		config.Interval = time.Minute
	}
	if config.ServiceName == "" {
		config.ServiceName = "postgresql"
	}
	if config.AgentID == "" {
		config.AgentID = "radar-agent"
	}
	return &receiverImpl{config: config, next: next, done: make(chan struct{}), prev: make(map[statementKey]statementSnapshot)}, nil
}

func (r *receiverImpl) Start(ctx context.Context, _ component.Host) error {
	db, err := sql.Open("pgx", r.config.DSN)
	if err != nil {
		return fmt.Errorf("open PostgreSQL QAN database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping PostgreSQL QAN database: %w", err)
	}
	r.db = db
	loopCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	go r.loop(loopCtx)
	return nil
}
func (r *receiverImpl) Shutdown(context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
	}
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
func (r *receiverImpl) loop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.collect(ctx)
		}
	}
}

func (r *receiverImpl) collect(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, digestQuery)
	if err != nil {
		return fmt.Errorf("query pg_stat_statements: %w", err)
	}
	defer rows.Close()
	current := make(map[statementKey]statementSnapshot)
	var bodies [][]byte
	for rows.Next() {
		var id int64
		var dbid uint32
		var query string
		var calls, total, min, max, mean, resultRows, hit, read float64
		if err := rows.Scan(&id, &dbid, &query, &calls, &total, &min, &max, &mean, &resultRows, &hit, &read); err != nil {
			return fmt.Errorf("scan pg_stat_statements row: %w", err)
		}
		key := statementKey{id, dbid}
		snap := statementSnapshot{calls, total, resultRows, hit, read}
		current[key] = snap
		r.mu.Lock()
		previous, ok := r.prev[key]
		r.mu.Unlock()
		if !ok {
			continue
		}
		delta := func(a, b float64) float64 {
			if a < b {
				return a
			}
			return a - b
		}
		count := delta(calls, previous.calls)
		if count <= 0 {
			continue
		}
		example := ""
		if r.config.IncludeExamples {
			example = query
		}
		bucket := map[string]any{"query_id": strconv.FormatInt(id, 10), "fingerprint": query, "service_name": r.config.ServiceName, "service_type": "postgresql", "agent_id": r.config.AgentID, "agent_type": "postgresql-pg-stat-statements", "period_start_unix_secs": uint32(time.Now().Add(-r.config.Interval).Unix()), "period_length_secs": uint32(r.config.Interval.Seconds()), "num_queries": count, "metrics": map[string]float64{"query_time_cnt": count, "query_time_sum": delta(total, previous.total) / 1000, "query_time_min": min / 1000, "query_time_max": max / 1000, "rows_sent_sum": delta(resultRows, previous.rows)}, "extra_metrics": map[string]float64{"mean_exec_time": mean / 1000, "shared_blks_hit_sum": delta(hit, previous.hit), "shared_blks_read_sum": delta(read, previous.read)}}
		if example != "" {
			bucket["example"] = example
		}
		body, err := json.Marshal(bucket)
		if err != nil {
			return err
		}
		bodies = append(bodies, body)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate pg_stat_statements rows: %w", err)
	}
	r.mu.Lock()
	r.prev = current
	r.mu.Unlock()
	for _, body := range bodies {
		if err := emitBucket(ctx, r.next, body, r.config.ServiceName); err != nil {
			return err
		}
	}
	return nil
}

func emitBucket(ctx context.Context, next consumer.Logs, body []byte, serviceName string) error {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutBool("radar.qan", true)
	rl.Resource().Attributes().PutStr("db.system", "postgresql")
	rl.Resource().Attributes().PutStr("service.name", serviceName)
	rec := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	rec.Body().SetStr(string(body))
	return next.ConsumeLogs(ctx, logs)
}

var _ receiver.Logs = (*receiverImpl)(nil)
var _ component.Component = (*receiverImpl)(nil)
