package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/control"
	"fiscalisation/edge-agent/device"
	"fiscalisation/edge-agent/journal"
	"fiscalisation/edge-agent/localapi"
	edgeruntime "fiscalisation/edge-agent/runtime"
	edgesync "fiscalisation/edge-agent/sync"
)

type config struct {
	Addr, DatabasePath, EdgeID, DeviceID, TenantID, RegisterID, SyncEndpoint, SyncKey, AuthToken, WebhookKey, AppEnv string
	DeviceAdapter, LocalAPIToken                                                                                     string
	SyncInterval, LeaseExpiresIn                                                                                     time.Duration
	FencingToken, OperationFrom, OperationTo, UNPFrom, UNPTo                                                         int64
	StorageQuotaBytes                                                                                                int64
	SimulatorReachable                                                                                               bool
}

func loadConfig(get func(string) string) (config, error) {
	c := config{Addr: value(get, "HTTP_ADDR", ":8082"), DatabasePath: value(get, "EDGE_DATABASE_PATH", "./data/edge.db"), EdgeID: get("EDGE_ID"), DeviceID: get("DEVICE_ID"), TenantID: value(get, "TENANT_ID", "00000000-0000-4000-8000-000000000001"), RegisterID: value(get, "REGISTER_ID", "00000000-0000-4000-8000-000000000001"), SyncEndpoint: get("FISCAL_EDGE_SYNC_URL"), SyncKey: get("EDGE_SYNC_HMAC_KEY"), AuthToken: get("FISCAL_AUTH_TOKEN"), WebhookKey: get("WEBHOOK_VERIFICATION_KEY"), AppEnv: value(get, "APP_ENV", "dev"), DeviceAdapter: get("DEVICE_ADAPTER"), LocalAPIToken: value(get, "EDGE_LOCAL_API_TOKEN", "dev-only-local-api-token"), SyncInterval: 2 * time.Second, LeaseExpiresIn: 24 * time.Hour, FencingToken: 1, OperationFrom: 1, OperationTo: 1000000, UNPFrom: 1, UNPTo: 1000000, StorageQuotaBytes: 1024 * 1024 * 1024, SimulatorReachable: !strings.EqualFold(get("SIMULATOR_DEVICE_REACHABLE"), "false")}
	if c.DeviceAdapter == "" {
		if strings.EqualFold(c.AppEnv, "prod") {
			c.DeviceAdapter = "unsupported"
		} else {
			c.DeviceAdapter = "simulator"
		}
	}
	if raw := get("SYNC_INTERVAL_MS"); raw != "" {
		ms, err := strconv.Atoi(raw)
		if err != nil || ms < 100 || ms > 300000 {
			return c, errors.New("invalid SYNC_INTERVAL_MS")
		}
		c.SyncInterval = time.Duration(ms) * time.Millisecond
	}
	for key, target := range map[string]*int64{"FENCING_TOKEN": &c.FencingToken, "OPERATION_FROM": &c.OperationFrom, "OPERATION_TO": &c.OperationTo, "UNP_FROM": &c.UNPFrom, "UNP_TO": &c.UNPTo} {
		if raw := get(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value < 1 {
				return c, errors.New("invalid " + key)
			}
			*target = value
		}
	}
	if raw := get("EDGE_STORAGE_QUOTA_BYTES"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1024*1024 {
			return c, errors.New("invalid EDGE_STORAGE_QUOTA_BYTES")
		}
		c.StorageQuotaBytes = value
	}
	if c.EdgeID == "" || c.DeviceID == "" || c.SyncEndpoint == "" || len(c.SyncKey) < 16 || len(c.WebhookKey) < 16 {
		return c, errors.New("missing or weak Edge configuration")
	}
	if c.OperationFrom > c.OperationTo || c.UNPFrom > c.UNPTo || (c.DeviceAdapter != "simulator" && c.DeviceAdapter != "unsupported") {
		return c, errors.New("invalid Edge authority or device adapter configuration")
	}
	if !strings.EqualFold(c.AppEnv, "prod") && len(c.LocalAPIToken) < 16 {
		return c, errors.New("weak EDGE_LOCAL_API_TOKEN")
	}
	if strings.EqualFold(c.AppEnv, "prod") && c.DeviceAdapter == "simulator" {
		return c, errors.New("simulator device adapter forbidden in PROD")
	}
	if strings.EqualFold(c.AppEnv, "prod") && (!strings.HasPrefix(c.SyncEndpoint, "https://") || len(c.SyncKey) < 32 || len(c.WebhookKey) < 32 || c.AuthToken == "") {
		return c, errors.New("unsafe PROD Edge configuration")
	}
	return c, nil
}

func value(get func(string) string, key, fallback string) string {
	if v := get(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	c, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(directory(c.DatabasePath), 0o750); err != nil {
		log.Fatal(err)
	}
	j, err := journal.Open(c.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer j.Close()
	uploader, err := edgesync.NewUploader(j, c.EdgeID, c.DeviceID, []byte(c.SyncKey), c.SyncEndpoint, c.AuthToken)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var finalDevice edgeruntime.Device
	switch c.DeviceAdapter {
	case "simulator":
		finalDevice = device.NewSimulator(c.SimulatorReachable)
	default:
		finalDevice = device.Unsupported{Capability: "HARDWARE_VENDOR_GATE_NOT_CLOSED"}
	}
	authorityManager := authority.New(authority.Lease{RegisterID: c.RegisterID, EdgeID: c.EdgeID, FencingToken: c.FencingToken, OperationFrom: c.OperationFrom, OperationTo: c.OperationTo, UNPFrom: c.UNPFrom, UNPTo: c.UNPTo, ExpiresAt: time.Now().UTC().Add(c.LeaseExpiresIn)})
	edgeRuntime := edgeruntime.New(j, authorityManager, finalDevice)
	edgeRuntime.SetStorageQuota(c.StorageQuotaBytes)
	mux := http.NewServeMux()
	mux.Handle("/", control.NewHandler(j, c.WebhookKey))
	if !strings.EqualFold(c.AppEnv, "prod") {
		binding := localapi.Binding{TenantID: c.TenantID, RegisterID: c.RegisterID, DeviceID: c.DeviceID, FencingToken: c.FencingToken}
		mux.Handle("/internal/v1/", localapi.New(edgeRuntime, binding, c.LocalAPIToken))
	}
	server := &http.Server{Addr: c.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	go syncLoop(ctx, uploader, c.SyncInterval)
	log.Printf("edge-agent listening on %s", c.Addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func syncLoop(ctx context.Context, uploader *edgesync.Uploader, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				uploaded, err := uploader.UploadOnce(ctx, 100)
				if err != nil {
					log.Printf("edge sync: %v", err)
					break
				}
				if !uploaded {
					break
				}
			}
		}
	}
}

func directory(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return "/"
	}
	return path[:i]
}
