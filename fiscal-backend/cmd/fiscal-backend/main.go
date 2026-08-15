package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fiscalisation/fiscal-backend/internal/api"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"fiscalisation/fiscal-backend/internal/emailworker"
	"fiscalisation/fiscal-backend/internal/mqttclient"
	"fiscalisation/fiscal-backend/internal/persistence"
	"fiscalisation/fiscal-backend/internal/startup"
	"fiscalisation/fiscal-backend/internal/webhook"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	repo := domain.Repository(domain.NewMemoryRepository())
	if cfg.DatabaseURL != "" {
		startupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		store, e := startup.Retry(startupContext, 500*time.Millisecond, func() (*persistence.Postgres, error) {
			return persistence.OpenWithReader(cfg.DatabaseURL, cfg.RLSDatabaseURL)
		})
		if e != nil {
			log.Fatal(e)
		}
		defer store.Close()
		persistent, e := domain.NewPersistentRepository(store)
		if e != nil {
			log.Fatal(e)
		}
		repo = persistent
	}
	var driver domain.Driver = domain.NewSimulatorWithCardTerminal(cfg.AllowStubAdapters, cfg.SimulatorCardTerminalAvailable)
	var mqttBridge *mqttclient.Bridge
	if cfg.EMQXBroker != "" {
		mqttBridge = mqttclient.NewBridge(cfg.BLESigningKey, repo)
		driver = mqttBridge
	}
	svc := domain.NewService(repo, driver)
	svc.SetRequireHardwareSyncSignatures(cfg.AppEnv == "prod" || cfg.EMQXBroker != "")
	svc.SetBLESigningKey(cfg.BLESigningKey)
	svc.SetLocalTokenTrust(cfg.LocalTokenIssuer, cfg.LocalTokenSigningKID, cfg.LocalTokenPublicKeyDERBase64)
	svc.SetSPADeploymentTrust(cfg.SPADeploymentDescriptorURL, cfg.SPADeploymentSigningKID, cfg.SPADeploymentPublicKeyDERBase64)
	if mqttBridge != nil {
		svc.SetCompositeBindingPublisher(mqttBridge)
	}
	if cfg.DeviceCACertFile != "" || cfg.DeviceCAKeyFile != "" {
		cert, certErr := os.ReadFile(cfg.DeviceCACertFile)
		if certErr != nil {
			log.Fatal(certErr)
		}
		key, keyErr := os.ReadFile(cfg.DeviceCAKeyFile)
		if keyErr != nil {
			log.Fatal(keyErr)
		}
		issuer, issuerErr := domain.NewX509DeviceCredentialIssuer(cert, key, cfg.DeviceMQTTTLSURI, cfg.DeviceMQTTWSSURI, cfg.BLESigningKey)
		if issuerErr != nil {
			log.Fatal(issuerErr)
		}
		svc.SetDeviceCredentialIssuer(issuer)
	}
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: api.NewHandler(svc, cfg), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mqttCleanup, err := mqttclient.Start(ctx, cfg, log.Default(), svc, mqttBridge)
	if err != nil {
		log.Fatal(err)
	}
	if mqttCleanup != nil {
		defer mqttCleanup()
	}
	go webhook.New(svc, cfg.WebhookTargetURL, cfg.WebhookSigningKey).Run(ctx)
	go emailworker.Run(ctx, cfg, log.Default())
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("fiscal-backend listening on %s env=%s", cfg.HTTPAddr, cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
