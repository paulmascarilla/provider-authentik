package authentik

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	goauthentik "goauthentik.io/api/v3"
)

const (
	errNewClient = "cannot create new authentik client"
)

func NewClient(endpoint string, token string) (*goauthentik.APIClient, error) {
	cfg := goauthentik.NewConfiguration()

	if strings.HasPrefix(endpoint, "http://") {
		cfg.Scheme = "http"
		cfg.Host = strings.TrimPrefix(endpoint, "http://")
	} else {
		cfg.Scheme = "https"
		cfg.Host = strings.TrimPrefix(endpoint, "https://")
	}

	cfg.AddDefaultHeader("Authorization", "Bearer "+token)
	cfg.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	return goauthentik.NewAPIClient(cfg), nil
}

func NewClientFromData(endpoint string, creds []byte) (*goauthentik.APIClient, error) {
	token := string(creds)

	if endpoint == "" || token == "" {
		return nil, errors.New("endpoint and token are required")
	}

	return NewClient(endpoint, token)
}
