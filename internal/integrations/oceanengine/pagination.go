package oceanengine

import "context"

func (c *Client) ListAll(ctx context.Context, start, end string, limit, maxPages int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 10
	}
	if maxPages <= 0 {
		maxPages = 1000
	}
	items := make([]map[string]any, 0)
	for page := 1; page <= maxPages; page++ {
		payload, err := c.ListPage(ctx, ListRequest{Start: start, End: end, Page: page, Limit: limit})
		if err != nil {
			return nil, err
		}
		data, _ := payload["data"].(map[string]any)
		batch, _ := data["ads"].([]any)
		for _, raw := range batch {
			if item, ok := raw.(map[string]any); ok {
				items = append(items, item)
			}
		}
		pagination, _ := data["pagination"].(map[string]any)
		total, _ := pagination["total_count"].(float64)
		totalPages, _ := pagination["total_page"].(float64)
		if len(batch) == 0 || (total > 0 && float64(len(items)) >= total) || totalPages == 0 || float64(page) >= totalPages {
			return items, nil
		}
	}
	return items, nil
}

func (c *Client) StatQueryAll(ctx context.Context, request StatQueryRequest, maxPages int) ([]map[string]any, error) {
	if request.Limit <= 0 {
		request.Limit = 500
	}
	if maxPages <= 0 {
		maxPages = 1000
	}
	rows := make([]map[string]any, 0)
	for page := 0; page < maxPages; page++ {
		request.Offset = page * request.Limit
		payload, err := c.StatQueryPage(ctx, request)
		if err != nil {
			return nil, err
		}
		data, _ := payload["data"].(map[string]any)
		stats, _ := data["StatsData"].(map[string]any)
		batch := FlattenRows(stats["Rows"])
		rows = append(rows, batch...)
		total, _ := stats["TotalCount"].(float64)
		if len(batch) == 0 || (total > 0 && float64(len(rows)) >= total) || len(batch) < request.Limit {
			return rows, nil
		}
	}
	return rows, nil
}
