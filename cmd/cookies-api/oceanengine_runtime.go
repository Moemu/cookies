package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
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
