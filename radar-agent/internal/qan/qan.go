package qan

import "encoding/json"

type CollectRequest struct {
	Buckets []Bucket `json:"buckets"`
}

type Bucket struct {
	QueryID             string             `json:"query_id"`
	Fingerprint         string             `json:"fingerprint"`
	ServiceName         string             `json:"service_name"`
	ServiceType         string             `json:"service_type"`
	Database            string             `json:"database,omitempty"`
	Schema              string             `json:"schema,omitempty"`
	Tables              []string           `json:"tables,omitempty"`
	Username            string             `json:"username,omitempty"`
	ClientHost          string             `json:"client_host,omitempty"`
	AgentID             string             `json:"agent_id"`
	AgentType           string             `json:"agent_type"`
	PeriodStartUnixSecs uint32             `json:"period_start_unix_secs"`
	PeriodLengthSecs    uint32             `json:"period_length_secs"`
	Example             string             `json:"example,omitempty"`
	ExampleTruncated    bool               `json:"example_truncated,omitempty"`
	NumQueries          float64            `json:"num_queries"`
	Metrics             Metrics            `json:"metrics,omitempty"`
	Labels              map[string]string  `json:"labels,omitempty"`
	ExtraMetrics        map[string]float64 `json:"extra_metrics,omitempty"`
}

type Metrics struct {
	QueryTimeCnt    float64 `json:"query_time_cnt,omitempty"`
	QueryTimeSum    float64 `json:"query_time_sum,omitempty"`
	QueryTimeMin    float64 `json:"query_time_min,omitempty"`
	QueryTimeMax    float64 `json:"query_time_max,omitempty"`
	QueryTimeP99    float64 `json:"query_time_p99,omitempty"`
	LockTimeSum     float64 `json:"lock_time_sum,omitempty"`
	RowsExaminedSum float64 `json:"rows_examined_sum,omitempty"`
	RowsSentSum     float64 `json:"rows_sent_sum,omitempty"`
}

func FixtureRequest(includeExample bool) CollectRequest {
	example := ""
	if includeExample {
		example = "select * from users where id = 123"
	}
	return CollectRequest{Buckets: []Bucket{{
		QueryID:             "7d1f1c4b8d",
		Fingerprint:         "select * from users where id = ?",
		ServiceName:         "mysql-prod-1",
		ServiceType:         "mysql",
		Database:            "app",
		Schema:              "",
		Tables:              []string{"users"},
		Username:            "app_user",
		ClientHost:          "10.0.0.10",
		AgentID:             "agent-1",
		AgentType:           "mysql-fixture",
		PeriodStartUnixSecs: 1720000000,
		PeriodLengthSecs:    60,
		Example:             example,
		ExampleTruncated:    false,
		NumQueries:          12,
		Metrics: Metrics{
			QueryTimeCnt:    12,
			QueryTimeSum:    3.4,
			QueryTimeMin:    0.01,
			QueryTimeMax:    1.2,
			QueryTimeP99:    1.1,
			LockTimeSum:     0.2,
			RowsExaminedSum: 1200,
			RowsSentSum:     24,
		},
		Labels:       map[string]string{"environment": "dev", "cluster": "fixture"},
		ExtraMetrics: map[string]float64{"rows_affected_sum": 0},
	}}}
}

func FixtureJSON(includeExample bool) ([]byte, error) {
	return json.Marshal(FixtureRequest(includeExample))
}
