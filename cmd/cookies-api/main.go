// cookies-api starts the shared platform HTTP surface.
//
// It deliberately exposes only operational endpoints and a request-context
// probe while the platform modules are being built. Business systems own their
// own HTTP surfaces under /api/{system}/v1.
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

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/httpserver"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	resolver, err := buildIdentityResolver(cfg)
	if err != nil {
		log.Fatalf("invalid identity configuration: %v", err)
	}
	db, err := database.Open(context.Background(), cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpserver.NewWithDependencies(httpserver.Dependencies{
			Resolver:          resolver,
			ProjectAuthorizer: buildProjectAuthorizer(cfg),
			Readiness:         database.Readiness{DB: db},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("cookies platform API listening on %s (%s)", cfg.HTTPAddr, cfg.Environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped unexpectedly: %v", err)
	}
}

func buildProjectAuthorizer(cfg config.Config) identity.ProjectAuthorizer {
	if cfg.LocalIdentity != nil {
		return identity.StaticProjectAuthorizer{ProjectID: contract.ProjectID(cfg.LocalIdentity.ProjectID)}
	}
	return identity.RejectingProjectAuthorizer{}
}

func buildIdentityResolver(cfg config.Config) (identity.Resolver, error) {
	if cfg.LocalIdentity == nil {
		return identity.RejectingResolver{}, nil
	}

	principalKind := contract.PrincipalKind(cfg.LocalIdentity.PrincipalKind)
	actor := contract.ActorContext{
		OrganizationID: contract.OrganizationID(cfg.LocalIdentity.OrganizationID),
		Principal: contract.Principal{
			Kind: principalKind,
			ID:   cfg.LocalIdentity.PrincipalID,
		},
		Scopes: contract.ScopesFromStrings(cfg.LocalIdentity.Scopes),
	}
	return identity.NewStaticResolver(actor)
}
