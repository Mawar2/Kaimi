package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GitHubRESTClient implements MCPClient using GitHub's REST API directly.
// This is simpler than the MCP protocol and works with standalone binaries.
type GitHubRESTClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewGitHubRESTClient creates a new GitHub REST API client.
func NewGitHubRESTClient(token string) *GitHubRESTClient {
	return &GitHubRESTClient{
		baseURL: "https://api.github.com",
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewGitHubRESTClientFromEnv creates a client using GITHUB_TOKEN env var.
func NewGitHubRESTClientFromEnv() (*GitHubRESTClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}
	return NewGitHubRESTClient(token), nil
}

// Call implements MCPClient interface by translating MCP tool calls to GitHub REST API calls.
func (c *GitHubRESTClient) Call(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error) {
	switch tool {
	case "mcp__github__list_issues":
		return c.listIssues(ctx, params)
	case "mcp__github__issue_read":
		return c.getIssue(ctx, params)
	case "mcp__github__search_pull_requests":
		return c.searchPullRequests(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported tool: %s", tool)
	}
}

// listIssues fetches issues from GitHub REST API.
func (c *GitHubRESTClient) listIssues(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	state, _ := params["state"].(string)
	perPage, _ := params["perPage"].(float64)

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	// Build URL
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=%s&per_page=%d",
		c.baseURL, owner, repo, strings.ToLower(state), int(perPage))

	// Add labels filter if provided
	if labels, ok := params["labels"].([]interface{}); ok && len(labels) > 0 {
		labelStrs := make([]string, len(labels))
		for i, label := range labels {
			labelStrs[i] = fmt.Sprintf("%v", label)
		}
		url += "&labels=" + strings.Join(labelStrs, ",")
	}

	// Make request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issues []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return in expected format (items array)
	return map[string]interface{}{
		"items": issues,
	}, nil
}

// getIssue fetches a specific issue from GitHub REST API.
func (c *GitHubRESTClient) getIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber, _ := params["issue_number"].(float64)

	if owner == "" || repo == "" || issueNumber == 0 {
		return nil, fmt.Errorf("owner, repo, and issue_number are required")
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d",
		c.baseURL, owner, repo, int(issueNumber))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issue map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return issue, nil
}

// searchPullRequests searches for PRs referencing an issue.
func (c *GitHubRESTClient) searchPullRequests(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

	// Build search query
	query := ""
	if q, ok := params["query"].(string); ok {
		query = q
	}

	url := fmt.Sprintf("%s/search/issues?q=%s+repo:%s/%s+type:pr",
		c.baseURL, query, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}
