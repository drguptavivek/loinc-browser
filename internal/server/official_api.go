package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultOfficialAPIBaseURL = "https://loinc.regenstrief.org/searchapi"

type OfficialSearchRequest struct {
	Scope               string `json:"scope"`
	Query               string `json:"query"`
	Rows                int    `json:"rows,omitempty"`
	Offset              int    `json:"offset,omitempty"`
	SortOrder           string `json:"sortorder,omitempty"`
	Language            int    `json:"language,omitempty"`
	IncludeFilterCounts bool   `json:"includefiltercounts,omitempty"`
	Username            string `json:"username,omitempty"`
	Password            string `json:"password,omitempty"`
	Remember            bool   `json:"remember,omitempty"`
	UseSavedCredentials bool   `json:"useSavedCredentials,omitempty"`
}

type OfficialSearchResponse struct {
	Scope          string                    `json:"scope"`
	Params         map[string]any            `json:"params"`
	UpstreamStatus int                       `json:"upstreamStatus"`
	Payload        any                       `json:"payload"`
	Local          *OfficialLocalIntegration `json:"local,omitempty"`
}

type OfficialLocalIntegration struct {
	Available bool                          `json:"available"`
	LOINCNums []string                      `json:"loincNums"`
	Matched   int                           `json:"matched"`
	Missing   int                           `json:"missing"`
	Matches   map[string]OfficialLocalMatch `json:"matches"`
	Message   string                        `json:"message,omitempty"`
}

type OfficialLocalMatch struct {
	LOINCNum string                    `json:"loincNum"`
	Found    bool                      `json:"found"`
	Term     *OfficialLocalTermSummary `json:"term,omitempty"`
	LocalURL string                    `json:"localUrl,omitempty"`
}

type OfficialLocalTermSummary struct {
	LOINCNum       string `json:"loincNum"`
	LongCommonName string `json:"longCommonName"`
	ShortName      string `json:"shortName"`
	Status         string `json:"status"`
	System         string `json:"system"`
	Class          string `json:"class"`
	Property       string `json:"property"`
	Scale          string `json:"scale"`
}

type officialSearchClient struct {
	baseURL    string
	httpClient *http.Client
}

func newOfficialSearchClient(baseURL string, httpClient *http.Client) *officialSearchClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultOfficialAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &officialSearchClient{baseURL: baseURL, httpClient: httpClient}
}

func (c *officialSearchClient) Search(ctx context.Context, req OfficialSearchRequest, credentials OfficialCredentials) (OfficialSearchResponse, int, error) {
	scopePath, ok := officialScopePath(req.Scope)
	if !ok {
		return OfficialSearchResponse{}, http.StatusBadRequest, fmt.Errorf("unsupported official API scope %q", req.Scope)
	}
	if strings.TrimSpace(req.Query) == "" {
		return OfficialSearchResponse{}, http.StatusBadRequest, errors.New("official API query is required")
	}
	if strings.TrimSpace(credentials.Username) == "" || credentials.Password == "" {
		return OfficialSearchResponse{}, http.StatusUnauthorized, errors.New("official API credentials are required")
	}

	values := url.Values{}
	values.Set("query", strings.TrimSpace(req.Query))
	if req.Rows > 0 {
		values.Set("rows", strconv.Itoa(req.Rows))
	}
	if req.Offset > 0 {
		values.Set("offset", strconv.Itoa(req.Offset))
	}
	if strings.TrimSpace(req.SortOrder) != "" {
		values.Set("sortorder", strings.TrimSpace(req.SortOrder))
	}
	if req.Language > 0 {
		values.Set("language", strconv.Itoa(req.Language))
	}
	if req.IncludeFilterCounts {
		values.Set("includefiltercounts", "true")
	}

	upstreamURL := c.baseURL + scopePath + "?" + values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return OfficialSearchResponse{}, http.StatusInternalServerError, fmt.Errorf("create official API request: %w", err)
	}
	httpReq.SetBasicAuth(credentials.Username, credentials.Password)
	httpReq.Header.Set("accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return OfficialSearchResponse{}, http.StatusBadGateway, fmt.Errorf("official API request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return OfficialSearchResponse{}, http.StatusBadGateway, fmt.Errorf("read official API response: %w", err)
	}
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if len(bytes.TrimSpace(body)) == 0 {
		payload = map[string]any{}
	} else if err := decoder.Decode(&payload); err != nil {
		payload = string(body)
	}
	envelope := OfficialSearchResponse{
		Scope:          normalizedOfficialScope(req.Scope),
		Params:         officialParamsEcho(req),
		UpstreamStatus: resp.StatusCode,
		Payload:        payload,
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return envelope, resp.StatusCode, errors.New("official API authentication failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return envelope, http.StatusBadGateway, fmt.Errorf("official API returned status %d", resp.StatusCode)
	}
	return envelope, http.StatusOK, nil
}

func officialScopePath(scope string) (string, bool) {
	switch normalizedOfficialScope(scope) {
	case "loincs":
		return "/loincs", true
	case "answerlists":
		return "/answerlists", true
	case "parts":
		return "/parts", true
	case "groups":
		return "/groups", true
	default:
		return "", false
	}
}

func normalizedOfficialScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "", "loinc", "loincs", "terms":
		return "loincs"
	case "answerlist", "answerlists", "answer-lists":
		return "answerlists"
	case "part", "parts":
		return "parts"
	case "group", "groups":
		return "groups"
	default:
		return scope
	}
}

func officialParamsEcho(req OfficialSearchRequest) map[string]any {
	params := map[string]any{
		"query": strings.TrimSpace(req.Query),
	}
	if req.Rows > 0 {
		params["rows"] = req.Rows
	}
	if req.Offset > 0 {
		params["offset"] = req.Offset
	}
	if strings.TrimSpace(req.SortOrder) != "" {
		params["sortorder"] = strings.TrimSpace(req.SortOrder)
	}
	if req.Language > 0 {
		params["language"] = req.Language
	}
	if req.IncludeFilterCounts {
		params["includefiltercounts"] = true
	}
	return params
}
