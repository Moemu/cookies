package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type creativeProductMediaImporter struct {
	uploads *assets.UploadService
	client  *http.Client
}

func (i creativeProductMediaImporter) ImportProductMedia(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, media []creative.AINativeRequirementMedia) ([]creative.AINativeRequirementMedia, error) {
	if i.uploads == nil {
		return nil, fmt.Errorf("asset upload service is required")
	}
	client := i.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			if !allowedDouyinProductImageURL(request.URL) {
				return fmt.Errorf("product image redirect is not allowed")
			}
			return nil
		}}
	}
	requestContext, ok := contract.RequestContextFrom(ctx)
	if !ok {
		requestContext = contract.RequestContext{RequestID: workspaceID, TraceID: workspaceID, Actor: actor}
	}
	requestContext.Actor = actor
	if !requestContext.Actor.HasScope("assets.write") {
		requestContext.Actor.Scopes = append(append([]contract.Scope{}, requestContext.Actor.Scopes...), contract.Scope("assets.write"))
	}
	result := append([]creative.AINativeRequirementMedia{}, media...)
	imported := make([]creative.AINativeRequirementMedia, 0, len(result))
	var importErrors []error
	for index := range result {
		item := result[index]
		parsed, err := url.Parse(item.URL)
		if err != nil || !allowedDouyinProductImageURL(parsed) {
			importErrors = append(importErrors, fmt.Errorf("product image URL is outside the approved commerce CDN"))
			continue
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			importErrors = append(importErrors, err)
			continue
		}
		response, err := client.Do(request)
		if err != nil {
			importErrors = append(importErrors, err)
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, assets.MaxImageBytes+1))
		response.Body.Close()
		if readErr != nil {
			importErrors = append(importErrors, readErr)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 || len(data) == 0 || int64(len(data)) > assets.MaxImageBytes {
			importErrors = append(importErrors, fmt.Errorf("product image download returned an invalid response"))
			continue
		}
		mimeType := http.DetectContentType(data[:min(len(data), 512)])
		if mimeType != "image/jpeg" && mimeType != "image/png" {
			importErrors = append(importErrors, fmt.Errorf("product image media type %q is unsupported", mimeType))
			continue
		}
		digest := sha256.Sum256(data)
		digestText := hex.EncodeToString(digest[:])
		identity := sha256.Sum256([]byte(string(projectID) + "\x00" + parsed.String()))
		filename := path.Base(parsed.Path)
		if filename == "." || filename == "/" || strings.TrimSpace(filename) == "" {
			filename = "product-image"
		}
		created, err := i.uploads.Create(ctx, requestContext, projectID, contract.IdempotencyKey("ai_native_product_"+hex.EncodeToString(identity[:16])), assets.CreateUploadRequest{
			Filename: filename, DeclaredMIMEType: mimeType, DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &digestText,
		})
		if err != nil {
			importErrors = append(importErrors, err)
			continue
		}
		session := created.Session
		if session.Status != assets.UploadSucceeded {
			if session.Status == assets.UploadCreated {
				if err = i.uploads.PutContent(ctx, requestContext.Actor, projectID, session.ID, bytes.NewReader(data), int64(len(data))); err != nil {
					importErrors = append(importErrors, err)
					continue
				}
			}
			session, err = i.uploads.Finalize(ctx, requestContext, projectID, session.ID)
			if err != nil {
				importErrors = append(importErrors, err)
				continue
			}
		}
		if session.ProjectAssetRef == nil {
			importErrors = append(importErrors, fmt.Errorf("product image import did not produce a stable asset"))
			continue
		}
		assetRef := session.ProjectAssetRef.AssetVersion
		item.AssetRef = &assetRef
		imported = append(imported, item)
	}
	return imported, errors.Join(importErrors...)
}

func allowedDouyinProductImageURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(value.Hostname(), "."))
	return host == "ecombdimg.com" || strings.HasSuffix(host, ".ecombdimg.com")
}
