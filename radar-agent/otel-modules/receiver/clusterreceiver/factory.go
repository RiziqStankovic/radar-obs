package clusterreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const typeStr = "cluster"

type Config struct {
	Interval       time.Duration `mapstructure:"interval"`
	ClusterID      string        `mapstructure:"cluster_id"`
	TenantID       string        `mapstructure:"tenant_id"`
	Ownership      string        `mapstructure:"ownership"`
	LeaseName      string        `mapstructure:"lease_name"`
	LeaseNamespace string        `mapstructure:"lease_namespace"`
	LeaseIdentity  string        `mapstructure:"lease_identity"`
}

func createDefaultConfig() component.Config {
	return &Config{Interval: time.Minute, Ownership: "leader", LeaseName: "radar-cluster-collector"}
}

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(component.MustNewType(typeStr), createDefaultConfig, xreceiver.WithLogs(createLogs, component.StabilityLevelAlpha))
}

func createLogs(_ context.Context, settings receiver.Settings, cfg component.Config, next consumer.Logs) (receiver.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid cluster config type %T", cfg)
	}
	if config.Interval <= 0 {
		config.Interval = time.Minute
	}
	if strings.TrimSpace(config.LeaseNamespace) == "" {
		config.LeaseNamespace = "default"
	}
	if strings.TrimSpace(config.LeaseName) == "" {
		config.LeaseName = "radar-cluster-collector"
	}
	if strings.TrimSpace(config.LeaseIdentity) == "" {
		config.LeaseIdentity = "radar-agent"
	}
	return &receiverImpl{config: *config, next: next, logger: settings.Logger}, nil
}

type receiverImpl struct {
	config Config
	next   consumer.Logs
	logger *zap.Logger
	client kubernetes.Interface
	cancel context.CancelFunc
	done   chan struct{}
}

func (r *receiverImpl) Start(ctx context.Context, _ component.Host) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("create in-cluster Kubernetes config: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}
	r.client = client
	loopCtx, cancel := context.WithCancel(ctx)
	r.cancel, r.done = cancel, make(chan struct{})
	go r.run(loopCtx)
	return nil
}

func (r *receiverImpl) Shutdown(context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.done != nil {
		<-r.done
	}
	return nil
}

func (r *receiverImpl) run(ctx context.Context) {
	defer close(r.done)
	if strings.EqualFold(r.config.Ownership, "leader") {
		lock, err := resourcelock.New(resourcelock.LeasesResourceLock,
			r.config.LeaseNamespace, r.config.LeaseName,
			r.client.CoreV1(), r.client.CoordinationV1(),
			resourcelock.ResourceLockConfig{Identity: r.config.LeaseIdentity})
		if err != nil {
			r.logError("create cluster leader lock", err)
			return
		}
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock: lock, ReleaseOnCancel: true, LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(leaderCtx context.Context) { r.collectLoop(leaderCtx) },
				OnStoppedLeading: func() {
					r.logError("cluster leadership lost", fmt.Errorf("lease %q no longer held", r.config.LeaseName))
				},
			},
		})
		return
	}
	r.collectLoop(ctx)
}

func (r *receiverImpl) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()
	if err := r.collect(ctx); err != nil {
		r.logError("initial cluster collection failed", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.collect(ctx); err != nil {
				r.logError("cluster collection failed", err)
			}
		}
	}
}

func (r *receiverImpl) collect(ctx context.Context) error {
	nodes, err := r.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	pods, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	namespaces, err := r.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	services, err := r.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	deployments, err := r.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	payload := map[string]any{"cluster_id": r.config.ClusterID, "tenant_id": r.config.TenantID, "collected_at": time.Now().UTC(), "nodes": len(nodes.Items), "pods": len(pods.Items), "namespaces": len(namespaces.Items), "services": len(services.Items), "deployments": len(deployments.Items)}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("radar.signal", "cluster")
	resourceLogs.Resource().Attributes().PutStr("cluster.id", r.config.ClusterID)
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(string(body))
	r.next.ConsumeLogs(ctx, logs)
	return nil
}

func (r *receiverImpl) logError(message string, err error) {
	if r.logger != nil {
		r.logger.Error(message, zap.Error(err))
	}
}

var _ receiver.Logs = (*receiverImpl)(nil)
var _ component.Component = (*receiverImpl)(nil)
