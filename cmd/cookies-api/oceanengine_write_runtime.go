package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/systems/insights"
)

type oceanEngineExternalAccountResolver interface {
	ResolveAccountIDByExternalID(context.Context, string, string, string) (string, error)
}

type oceanEngineAccountSessionAuthMarker interface {
	MarkAccountSessionAuthRequired(context.Context, string, string, int64, time.Time) (connector.OceanEngineAccountSession, error)
}

type oceanEngineConnectorWriterFactory struct {
	accountSessions connector.AccountSessionStore
	authMarker      oceanEngineAccountSessionAuthMarker
	accounts        oceanEngineExternalAccountResolver
	cipher          insights.SecretCipher
	baseURL         string
	client          *http.Client
	tokenCache      *oceanengine.CSRFTokenCache
}

type oceanEngineConnectorWriter struct {
	Client         *oceanengine.WriteClient
	OrganizationID string
	AccountID      string
	SessionVersion int64
	factory        oceanEngineConnectorWriterFactory
}

func (f oceanEngineConnectorWriterFactory) Open(ctx context.Context, run browserautomation.BrowserRpaRun) (*oceanEngineConnectorWriter, func(), error) {
	if f.accountSessions == nil || f.accounts == nil || f.cipher == nil {
		return nil, nil, fmt.Errorf("Ocean Engine write session access is not configured")
	}
	accountID, err := f.accounts.ResolveAccountIDByExternalID(ctx, string(run.OrganizationID), string(run.ProjectID), run.AccountID)
	if err != nil {
		return nil, nil, browserautomation.ErrAccountMismatch
	}
	session, err := f.accountSessions.GetAccountSession(ctx, string(run.OrganizationID), accountID)
	if err != nil || session.Status != connector.AccountSessionReady {
		return nil, nil, browserautomation.ErrEnvironmentUnavailable
	}
	plaintext, err := f.cipher.Decrypt(session.SessionCiphertext, session.SessionKeyVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt Ocean Engine Connector session")
	}
	client, clientErr := oceanengine.NewWriteClient(f.baseURL, run.AccountID, session.Version, oceanengine.Session{Cookies: string(plaintext)}, f.client, f.tokenCache)
	for index := range plaintext {
		plaintext[index] = 0
	}
	if clientErr != nil {
		return nil, nil, clientErr
	}
	writer := &oceanEngineConnectorWriter{Client: client, OrganizationID: string(run.OrganizationID), AccountID: accountID, SessionVersion: session.Version, factory: f}
	cleanup := func() { client.Close() }
	return writer, cleanup, nil
}

func (w *oceanEngineConnectorWriter) MarkAuthRequired(ctx context.Context, now time.Time) error {
	if w == nil || w.factory.authMarker == nil {
		return fmt.Errorf("Ocean Engine session auth marker is not configured")
	}
	_, err := w.factory.authMarker.MarkAccountSessionAuthRequired(ctx, w.OrganizationID, w.AccountID, w.SessionVersion, now.UTC())
	return err
}
