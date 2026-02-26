package clients

import (
	"context"
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	authentik "goauthentik.io/api/v3"
)

type Config struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

func NewClient(ctx context.Context, data []byte) (*authentik.APIClient, error) {

	config := &Config{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal authentik credentials")
	}

	// Valider les champs requis
	if config.Endpoint == "" {
		return nil, errors.New("endpoint cannot be empty")
	}

	if config.Token == "" {
		return nil, errors.New("token cannot be empty")
	}

	configuration := authentik.NewConfiguration()
	configuration.Host = config.Endpoint
	configuration.AddDefaultHeader("Authorization", "Bearer "+config.Token)

	return authentik.NewAPIClient(configuration), nil
}
