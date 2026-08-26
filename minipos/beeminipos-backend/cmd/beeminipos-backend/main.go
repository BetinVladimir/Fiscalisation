package main

import (
	"context"
	"errors"
	"fiscalisation/beeminipos-backend/internal/api"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/emailauth"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"fiscalisation/beeminipos-backend/internal/migrations"
	"fiscalisation/beeminipos-backend/internal/mqttclient"
	"fiscalisation/beeminipos-backend/internal/persistence"
	"fiscalisation/beeminipos-backend/internal/startup"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	c := config.Load()
	if e := c.Validate(); e != nil {
		log.Fatal(e)
	}
	var svc *domain.Service
	if c.DatabaseURL != "" {
		startupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, e := startup.Retry(startupContext, 500*time.Millisecond, func() (struct{}, error) {
			return struct{}{}, migrations.Run(startupContext, c.DatabaseURL)
		}); e != nil {
			log.Fatal(e)
		}
		store, e := startup.Retry(startupContext, 500*time.Millisecond, func() (*persistence.Postgres, error) {
			return persistence.OpenWithReader(c.DatabaseURL, c.RLSDatabaseURL)
		})
		if e != nil {
			log.Fatal(e)
		}
		defer store.Close()
		svc, e = domain.NewPersistentService(c.FiscalBaseURL, c.APIVersion, store)
		if e != nil {
			log.Fatal(e)
		}
	} else {
		svc = domain.NewService(c.FiscalBaseURL, c.APIVersion)
	}
	if c.OAuthTokenURL != "" {
		svc.SetFiscalAuthProvider(domain.NewClientCredentialsProvider(c.OAuthTokenURL, c.OAuthClientID, c.OAuthClientSecret, c.OAuthScope, c.OAuthAudience, nil))
	} else {
		svc.SetFiscalAuthToken(c.FiscalAuthToken)
	}
	authService, e := emailauth.New(context.Background(), c.DatabaseURL, c.AuthHMACKey, c.AuthTokenIssuer, svc)
	if e != nil {
		log.Fatal(e)
	}
	defer authService.Close()
	authService.ConfigureSMTP(c.SMTPHost, c.SMTPPort, c.SMTPUser, c.SMTPPassword, c.SMTPFrom, c.SMTPAllowPlaintext)
	var fiscalClient *fiscalintegration.Client
	if c.DatabaseURL != "" && c.FiscalSystemToken != "" && c.FiscalCredentialEncryptionKeyBase64 != "" {
		fiscalClient, e = fiscalintegration.New(c.DatabaseURL, c.FiscalBaseURL, c.FiscalSystemToken, c.FiscalCredentialEncryptionKeyBase64, c.FiscalCredentialKEKID)
		if e != nil {
			log.Fatal(e)
		}
		svc.SetFiscalReadinessCheck(func(tenant, register string) bool {
			return fiscalClient.ReadyForFiscalOperations(context.Background(), tenant, register)
		})
		defer fiscalClient.Close()
	}
	h := api.NewWithIntegrations(svc, c, authService, fiscalClient)
	s := newHTTPServer(c.HTTPAddr, h)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go authService.RunEmailWorker(ctx)
	mqttCleanup, err := mqttclient.Start(ctx, c, log.Default())
	if err != nil {
		log.Fatal(err)
	}
	if mqttCleanup != nil {
		defer mqttCleanup()
	}
	go func() {
		<-ctx.Done()
		x, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(x)
	}()
	if e := s.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
