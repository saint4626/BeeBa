package meilisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	searchdomain "beeba.org/internal/domain/search"
)

type Client struct {
	baseURL    string
	apiKey     string
	indexName  string
	httpClient *http.Client
	ensureMu   sync.Mutex
	ensured    bool
}

func New(baseURL string, apiKey string, indexName string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("meilisearch url is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("meilisearch url is invalid: %w", err)
	}
	indexName = strings.TrimSpace(indexName)
	if indexName == "" {
		return nil, fmt.Errorf("meilisearch index name is required")
	}
	return &Client{
		baseURL:   baseURL,
		apiKey:    strings.TrimSpace(apiKey),
		indexName: indexName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) EnsureContentIndex(ctx context.Context) error {
	c.ensureMu.Lock()
	defer c.ensureMu.Unlock()
	if c.ensured {
		return nil
	}
	settings := map[string]any{
		"searchableAttributes": []string{
			"title",
			"description",
			"category_name",
			"author_username",
			"author_name",
			"tags",
			"tag_names",
		},
		"filterableAttributes": []string{
			"category_slug",
			"tags",
			"nsfw",
			"author_id",
		},
		"sortableAttributes": []string{
			"published_at_unix",
			"likes_count",
			"downloads_count",
		},
		"displayedAttributes": []string{
			"id",
			"slug",
			"title",
			"description",
			"category_slug",
			"category_name",
			"author_id",
			"author_username",
			"author_name",
			"tags",
			"tag_names",
			"nsfw",
			"preview_image_id",
			"likes_count",
			"downloads_count",
			"comments_count",
			"published_at",
			"published_at_unix",
			"updated_at",
		},
	}
	if err := c.doJSON(ctx, http.MethodPatch, c.indexPath()+"/settings", settings, nil); err != nil {
		return err
	}
	c.ensured = true
	return nil
}

func (c *Client) UpsertContent(ctx context.Context, doc searchdomain.ContentDocument) error {
	if err := c.EnsureContentIndex(ctx); err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, c.indexPath()+"/documents?primaryKey=id", []searchdomain.ContentDocument{doc}, nil)
}

func (c *Client) DeleteContent(ctx context.Context, contentID string) error {
	if err := c.EnsureContentIndex(ctx); err != nil {
		return err
	}
	path := c.indexPath() + "/documents/" + url.PathEscape(contentID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) SearchContent(ctx context.Context, query searchdomain.ContentQuery) (searchdomain.ContentSearchResult, error) {
	if err := c.EnsureContentIndex(ctx); err != nil {
		return searchdomain.ContentSearchResult{}, err
	}
	request := map[string]any{
		"q":                    query.Query,
		"limit":                query.Limit,
		"offset":               query.Offset,
		"attributesToRetrieve": []string{"id"},
		"matchingStrategy":     "last",
	}
	filter := contentFilters(query)
	if filter != "" {
		request["filter"] = filter
	}
	if sort := contentSort(query.Sort); sort != "" {
		request["sort"] = []string{sort}
	}

	var response struct {
		Hits []struct {
			ID string `json:"id"`
		} `json:"hits"`
		EstimatedTotalHits int `json:"estimatedTotalHits"`
		Limit              int `json:"limit"`
		Offset             int `json:"offset"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.indexPath()+"/search", request, &response); err != nil {
		return searchdomain.ContentSearchResult{}, err
	}
	ids := make([]string, 0, len(response.Hits))
	for _, hit := range response.Hits {
		if strings.TrimSpace(hit.ID) != "" {
			ids = append(ids, hit.ID)
		}
	}
	return searchdomain.ContentSearchResult{
		IDs:                ids,
		EstimatedTotalHits: response.EstimatedTotalHits,
		Limit:              response.Limit,
		Offset:             response.Offset,
	}, nil
}

func (c *Client) indexPath() string {
	return c.baseURL + "/indexes/" + url.PathEscape(c.indexName)
}

func contentFilters(query searchdomain.ContentQuery) string {
	filters := make([]string, 0, 3)
	if !query.IncludeNSFW {
		filters = append(filters, "nsfw = false")
	}
	if query.CategorySlug != "" {
		filters = append(filters, "category_slug = "+strconv.Quote(query.CategorySlug))
	}
	if len(query.Tags) > 0 {
		quoted := make([]string, 0, len(query.Tags))
		for _, tag := range query.Tags {
			quoted = append(quoted, strconv.Quote(tag))
		}
		filters = append(filters, "tags IN ["+strings.Join(quoted, ", ")+"]")
	}
	return strings.Join(filters, " AND ")
}

func contentSort(sort string) string {
	switch sort {
	case "likes":
		return "likes_count:desc"
	case "downloads":
		return "downloads_count:desc"
	default:
		return "published_at_unix:desc"
	}
}

func (c *Client) doJSON(ctx context.Context, method string, endpoint string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal meilisearch request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("create meilisearch request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call meilisearch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if method == http.MethodDelete && resp.StatusCode == http.StatusNotFound {
			io.Copy(io.Discard, resp.Body)
			return nil
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("meilisearch returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if target == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode meilisearch response: %w", err)
	}
	return nil
}
