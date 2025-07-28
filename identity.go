package conohav3

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const identityBaseURL = "https://identity.%s.conoha.io"

type identifier struct {
	baseURL    *url.URL
	HTTPClient *http.Client
}

// newIdentifier creates a new Identifier.
func newIdentifier(region string) (*identifier, error) {
	if region == "" {
		region = "c3j1"
	}

	baseURL, err := url.Parse(fmt.Sprintf(identityBaseURL, region))
	if err != nil {
		return nil, err
	}

	return &identifier{
		baseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// getToken returns a x-subject-token from Identity API.
// https://doc.conoha.jp/reference/api-vps3/api-identity-vps3/identity-post_tokens-v3/?btn_id=reference-api-guideline-v3--sidebar_reference-identity-post_tokens-v3
func (c *identifier) getToken(ctx context.Context, APITenantID, APIUserID, APIPassword string) (string, error) {
	auth := auth{
		Identity: identity{
			Methods: []string{"password"},
			Password: password{
				User: user{
					ID:       APIUserID,
					Password: APIPassword,
				},
			},
		},
		Scope: scope{
			Project: project{
				ID: APITenantID,
			},
		},
	}
	endpoint := c.baseURL.JoinPath("v3", "auth", "tokens")

	req, err := newJSONRequest(ctx, http.MethodPost, endpoint, &identityRequest{Auth: auth})
	if err != nil {
		return "", err
	}

	return c.do(req)
}

// do sends a request and returns a token from x-subject-token header.
func (c *identifier) do(req *http.Request) (string, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("got invalid status: HTTP %d", resp.StatusCode)
	}

	token := resp.Header.Get("x-subject-token")
	if token == "" {
		return "", fmt.Errorf("x-subject-token header is missing in response")
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	return token, nil
}
