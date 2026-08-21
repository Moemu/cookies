package oceanengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ListRequest struct {
	Start string
	End   string
	Page  int
	Limit int
}

type StatQueryRequest struct {
	DatasetKey string
	Dimensions []string
	Metrics    []string
	StartTime  string
	EndTime    string
	Offset     int
	Limit      int
	Host       string
	Extra      map[string]any
}

type Reader interface {
	ListPage(context.Context, ListRequest) (map[string]any, error)
	PromotionConfiguration(context.Context, string) (map[string]any, error)
	PromotionMaterials(context.Context, string, bool) (map[string]any, error)
	Attributes(context.Context, []string, string) (map[string]any, error)
	StatQueryPage(context.Context, StatQueryRequest) (map[string]any, error)
	AccountInfo(context.Context) (map[string]any, error)
}

func (c *Client) ListPage(ctx context.Context, request ListRequest) (map[string]any, error) {
	body := map[string]any{
		"st": request.Start, "et": request.End, "page": request.Page, "limit": request.Limit,
		"sort_stat": "create_time", "project_status": []int{-1}, "promotion_status": []int{-1},
		"sort_order": 1, "campaign_type": []int{1}, "fields": []string{"stat_cost", "show_cnt", "click_cnt", "convert_cnt"},
		"isSophonx": 1, "project_ids": []string{}, "cascade_fields": []string{"disable_by_cpl2"}, "metrics_range_filter": []any{},
	}
	return c.postJSON(ctx, "/ad/api/promotion/ads/list", body)
}

func (c *Client) PromotionConfiguration(ctx context.Context, promotionID string) (map[string]any, error) {
	return c.getJSON(ctx, "/ad/api/promotion/ads/get_promotion_detail?promotion_ids="+promotionID)
}

func (c *Client) PromotionMaterials(ctx context.Context, promotionID string, needGroup bool) (map[string]any, error) {
	path := "/superior/api/ad/promotion/detail?promotion_ids=" + promotionID + "&need_invisible_material=false&need_material_group=" + fmt.Sprintf("%t", needGroup)
	return c.getJSON(ctx, path)
}

func (c *Client) Attributes(ctx context.Context, promotionIDs []string, requestID string) (map[string]any, error) {
	body := map[string]any{"promotion_ids": promotionIDs, "cascade_fields": []string{"diagnosis", "diagnosis_interfere_status", "compensate_status"}, "need_trans_toLocal": true, "ad_list_request_id": requestID}
	return c.postJSON(ctx, "/ad/api/promotion/ads/attribute/list", body)
}

func (c *Client) StatQueryPage(ctx context.Context, request StatQueryRequest) (map[string]any, error) {
	path := "/report/api/tool/agw/statistics_sophonx/statQuery"
	if request.Host == "ad" {
		path = "/ad/api/agw/statistics_sophonx/statQuery"
	}
	body := map[string]any{
		"DataSetKey": request.DatasetKey, "Dimensions": request.Dimensions, "StartTime": request.StartTime, "EndTime": request.EndTime,
		"Filters":    map[string]any{"ConditionRelationshipType": 1, "Conditions": []any{map[string]any{"Field": "advertiser_id", "Operator": 7, "Values": []string{c.AdvertiserID}}}},
		"IsDownload": false, "Metrics": request.Metrics, "PageParams": map[string]int{"Limit": request.Limit, "Offset": request.Offset},
	}
	if len(request.Dimensions) > 0 {
		body["OrderBy"] = []any{map[string]any{"Field": request.Dimensions[0], "Type": 2}}
	}
	if request.Extra != nil {
		body["Extra"] = request.Extra
	}
	return c.postJSON(ctx, path, body)
}

func (c *Client) AccountInfo(ctx context.Context) (map[string]any, error) {
	paths := []string{"/ad/api/account/info", "/superior/api/v2/account/info", "/ad/api/account/conf"}
	var value map[string]any
	var err error
	for index, path := range paths {
		value, err = c.getJSON(ctx, path)
		if err == nil || index == len(paths)-1 || !accountInfoFallbackAllowed(err) {
			return value, err
		}
	}
	return value, err
}

func accountInfoFallbackAllowed(err error) bool {
	var statusErr HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusMethodNotAllowed
	}
	var redirectErr RedirectBlockedError
	return errors.As(err, &redirectErr)
}

func (c *Client) GlobalInfo(ctx context.Context) (map[string]any, error) {
	return c.getJSON(ctx, "/api/ebp/ebp_info/get_global_info")
}

func (c *Client) getJSON(ctx context.Context, path string) (map[string]any, error) {
	return c.do(ctx, http.MethodGet, path, nil, "")
}
func (c *Client) postJSON(ctx context.Context, path string, value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, bytes.NewReader(encoded), "application/json;charset=UTF-8")
}

func FlattenRows(rows any) []map[string]any {
	var leaves []map[string]any
	walkRows(rows, &leaves)
	return leaves
}
func walkRows(value any, leaves *[]map[string]any) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if nested, ok := row["Rows"]; ok {
			walkRows(nested, leaves)
			continue
		}
		if _, ok := row["Metrics"]; ok {
			*leaves = append(*leaves, row)
		}
	}
}
