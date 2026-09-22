package ingest

import (
	"fmt"
	"strings"

	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/storage"
)

type QANCollectRequest struct {
	Buckets []storage.QANBucket `json:"buckets"`
}

func (r QANCollectRequest) Validate() error {
	if len(r.Buckets) == 0 {
		return fmt.Errorf("buckets are required")
	}
	for i, bucket := range r.Buckets {
		if err := validateQANBucket(bucket); err != nil {
			return fmt.Errorf("bucket %d: %w", i, err)
		}
	}
	return nil
}

func validateQANBucket(b storage.QANBucket) error {
	required := map[string]string{
		"query_id":     b.QueryID,
		"fingerprint":  b.Fingerprint,
		"service_name": b.ServiceName,
		"service_type": b.ServiceType,
		"agent_id":     b.AgentID,
		"agent_type":   b.AgentType,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	if b.PeriodStartUnixSecs == 0 {
		return fmt.Errorf("period_start_unix_secs is required")
	}
	if b.PeriodLengthSecs == 0 {
		return fmt.Errorf("period_length_secs is required")
	}
	if b.NumQueries <= 0 {
		return fmt.Errorf("num_queries must be greater than zero")
	}
	return nil
}
