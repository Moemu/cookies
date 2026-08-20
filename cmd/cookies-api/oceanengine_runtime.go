package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

type oceanEngineSessionVerifier struct {
	baseURL string
	client  *http.Client
}

func (v oceanEngineSessionVerifier) VerifyOceanEngineSession(ctx context.Context, session []byte) error {
	if strings.TrimSpace(string(session)) == "" {
		return oceanengine.ErrSessionInvalid
	}
	client, err := oceanengine.NewSessionClient(v.baseURL, oceanengine.Session{Cookies: string(session)}, v.client)
	if err != nil {
		return err
	}
	client.Delay = 0
	_, err = client.GlobalInfo(ctx)
	if err != nil {
		return fmt.Errorf("ocean engine session verification failed")
	}
	return nil
}

var _ insights.OceanEngineSessionVerifier = oceanEngineSessionVerifier{}

type oceanEngineConnectorReaderFactory struct {
	sessions insights.OceanEngineSessionRepository
	cipher   insights.SecretCipher
	baseURL  string
	client   *http.Client
	accounts interface {
		ResolveExternalAccountID(context.Context, string, string, string) (string, error)
	}
}

func (f oceanEngineConnectorReaderFactory) Open(ctx context.Context, request connector.SyncRequest) (oceanengine.Reader, func(), error) {
	if f.sessions == nil || f.cipher == nil {
		return nil, nil, fmt.Errorf("Ocean Engine session access is not configured")
	}
	session, err := f.sessions.GetProjectOceanEngineSession(ctx, contract.OrganizationID(request.OrganizationID), contract.ProjectID(request.ProjectID))
	if err != nil {
		return nil, nil, err
	}
	if session.Status != insights.OceanEngineSessionReady {
		return nil, nil, fmt.Errorf("Ocean Engine session is not ready")
	}
	plaintext, err := f.cipher.Decrypt(session.SessionCiphertext, session.SessionKeyVersion)
	if err != nil {
		return nil, nil, err
	}
	if f.accounts == nil {
		for index := range plaintext {
			plaintext[index] = 0
		}
		return nil, nil, fmt.Errorf("Ocean Engine account catalog is not configured")
	}
	externalID, resolveErr := f.accounts.ResolveExternalAccountID(ctx, request.OrganizationID, request.ProjectID, request.AccountRef)
	if resolveErr != nil {
		for index := range plaintext {
			plaintext[index] = 0
		}
		return nil, nil, resolveErr
	}
	client, err := oceanengine.NewClient(f.baseURL, externalID, oceanengine.Session{Cookies: string(plaintext)}, f.client)
	for index := range plaintext {
		plaintext[index] = 0
	}
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { client.Session.Cookies = ""; client.Session.CSRFToken = "" }
	return client, cleanup, nil
}

var _ connector.ReaderFactory = oceanEngineConnectorReaderFactory{}

type oceanEngineAccountProbe struct {
	sessions insights.OceanEngineSessionRepository
	cipher   insights.SecretCipher
	baseURL  string
	client   *http.Client
}

func (p oceanEngineAccountProbe) Verify(ctx context.Context, organizationID, projectID, externalID string) error {
	session, err := p.sessions.GetProjectOceanEngineSession(ctx, contract.OrganizationID(organizationID), contract.ProjectID(projectID))
	if err != nil {
		return err
	}
	if session.Status != insights.OceanEngineSessionReady {
		return fmt.Errorf("Ocean Engine session is not ready")
	}
	plaintext, err := p.cipher.Decrypt(session.SessionCiphertext, session.SessionKeyVersion)
	if err != nil {
		return err
	}
	client, err := oceanengine.NewClient(p.baseURL, externalID, oceanengine.Session{Cookies: string(plaintext)}, p.client)
	for index := range plaintext {
		plaintext[index] = 0
	}
	if err != nil {
		return err
	}
	defer func() { client.Session.Cookies = ""; client.Session.CSRFToken = "" }()
	_, err = client.AccountInfo(ctx)
	if err != nil {
		return fmt.Errorf("Ocean Engine account verification failed")
	}
	return nil
}

var _ connector.AccountProbe = oceanEngineAccountProbe{}
