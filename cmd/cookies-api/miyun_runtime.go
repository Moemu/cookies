package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/shikanon/cookies/internal/integrations/crawler"
	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

type miyunProtocolAdapter struct {
	endpoint string
	cipher   insights.MiyunSecretCipher
	client   *http.Client
	gate     *crawler.YouShuGate
}

func (a miyunProtocolAdapter) FetchMiyunPage(ctx context.Context, connection insights.MiyunConnection, operation string, query crawler.YouShuQuery) (crawler.YouShuPage, error) {
	client, plaintext, err := a.clientFor(connection)
	if err != nil {
		return crawler.YouShuPage{}, err
	}
	defer clearBytes(plaintext)
	if operation == "cid" {
		return client.CID(ctx, query)
	}
	return client.Product(ctx, query)
}

func (a miyunProtocolAdapter) VerifyMiyunConnection(ctx context.Context, session []byte) error {
	client, err := crawler.NewYouShuClient(a.endpoint, string(session), a.client)
	if err != nil {
		return err
	}
	client.Gate = a.gate
	now := time.Now().UTC()
	_, err = client.Product(ctx, crawler.YouShuQuery{
		MaterialIDs: []string{}, StartDate: now.AddDate(0, 0, -1).Format("2006-01-02"), EndDate: now.Format("2006-01-02"),
		Keyword: "连接验证", Page: 1, Order: "impression_desc", IsExact: crawler.YouShuBool(false), ProductID: []string{},
		Tpl: []string{}, SearchField: "all", SearchDSL: nil, AccountType: []string{}, IsSearchAiScene: crawler.YouShuInt(0),
	})
	return err
}

func (a miyunProtocolAdapter) clientFor(connection insights.MiyunConnection) (*crawler.YouShuClient, []byte, error) {
	plaintext, err := a.cipher.Decrypt(connection.SessionCiphertext, connection.SessionKeyVersion)
	if err != nil {
		return nil, nil, err
	}
	client, err := crawler.NewYouShuClient(a.endpoint, string(plaintext), a.client)
	if err != nil {
		clearBytes(plaintext)
		return nil, nil, err
	}
	client.Gate = a.gate
	return client, plaintext, nil
}

type miyunAuthorizedImportAdapter struct {
	downloader *crawler.YouShuDownloader
	assets     assets.ExternalImportService
	ledger     assets.ExternalImportRepository
	workRoot   string
}

func (a miyunAuthorizedImportAdapter) ImportMiyunMaterial(ctx context.Context, request insights.MiyunAuthorizedImportRequest) (insights.MiyunAuthorizedImportResult, error) {
	if a.downloader == nil || a.ledger == nil || request.MiyunMaterialID == "" {
		return insights.MiyunAuthorizedImportResult{}, fmt.Errorf("Miyun authorized import is unavailable")
	}
	if err := os.MkdirAll(a.workRoot, 0o700); err != nil {
		return insights.MiyunAuthorizedImportResult{}, err
	}
	temporary, err := os.CreateTemp(a.workRoot, "miyun-download-*.mp4")
	if err != nil {
		return insights.MiyunAuthorizedImportResult{}, err
	}
	name := temporary.Name()
	defer os.Remove(name)
	result, downloadErr := a.downloader.Download(ctx, request.ResourceURL, request.ExpectedSize, temporary)
	closeErr := temporary.Close()
	if downloadErr != nil {
		return insights.MiyunAuthorizedImportResult{}, downloadErr
	}
	if closeErr != nil {
		return insights.MiyunAuthorizedImportResult{}, closeErr
	}
	sourceLocator := "miyun://material/" + request.MiyunMaterialID
	if request.SourceRefStatus == "verified" && request.SourceRef != "" {
		sourceLocator = request.SourceRef
	}
	requestContext := contract.RequestContext{RequestID: "miyun-import-" + request.MaterialID, TraceID: "miyun-import-" + request.MaterialID, Actor: request.Actor}
	ref, err := a.assets.Import(ctx, requestContext, request.ProjectID, contract.IdempotencyKey(request.IdempotencyKey), assets.ExternalMediaImportRequest{
		SourceProvider: "miyun", SourceObjectID: request.MiyunMaterialID, SourceLocator: sourceLocator,
		MIMEType: "video/mp4", SizeBytes: result.SizeBytes,
	}, func(context.Context) (io.ReadCloser, error) { return os.Open(name) })
	if err != nil {
		return insights.MiyunAuthorizedImportResult{}, err
	}
	ledger, err := a.ledger.GetExternalImportBySource(ctx, request.Actor.OrganizationID, request.ProjectID, "miyun", request.MiyunMaterialID)
	if err != nil {
		return insights.MiyunAuthorizedImportResult{}, err
	}
	return insights.MiyunAuthorizedImportResult{ExternalImportID: ledger.ID, AssetRef: ref.AssetVersion, ContentSHA256: result.SHA256}, nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func miyunWorkRoot(base string) string {
	return filepath.Join(base, "miyun-downloads")
}
