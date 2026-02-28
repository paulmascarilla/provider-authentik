package application

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	goauthentik "goauthentik.io/api/v3"
)

const (
	errObserveApp  = "cannot observe Application"
	errCreateApp   = "cannot create Application"
	errUpdateApp   = "cannot update Application"
	errDeleteApp   = "cannot delete Application"
	errAppNotFound = "Application not found"
)

type ApplicationService interface {
	Get(ctx context.Context, slug string) (*goauthentik.Application, error)
	Create(ctx context.Context, app *goauthentik.ApplicationRequest) (*goauthentik.Application, error)
	Update(ctx context.Context, slug string, app *goauthentik.ApplicationRequest) (*goauthentik.Application, error)
	CreateOrUpdate(ctx context.Context, app *goauthentik.ApplicationRequest) (*goauthentik.Application, error)
	Delete(ctx context.Context, slug string) error
}

type authentikService struct {
	client *goauthentik.APIClient
}

func NewService(client *goauthentik.APIClient) ApplicationService {
	return &authentikService{client: client}
}

func (s *authentikService) Get(ctx context.Context, slug string) (*goauthentik.Application, error) {
	apps, _, err := s.client.CoreApi.CoreApplicationsList(ctx).Slug(slug).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, errors.Wrap(err, errObserveApp)
	}

	if len(apps.Results) == 0 {
		return nil, nil
	}

	app := apps.Results[0]
	return &app, nil
}

func (s *authentikService) Create(ctx context.Context, req *goauthentik.ApplicationRequest) (*goauthentik.Application, error) {
	app, _, err := s.client.CoreApi.CoreApplicationsCreate(ctx).ApplicationRequest(*req).Execute()
	if err != nil {
		return nil, errors.Wrap(err, errCreateApp)
	}
	return app, nil
}

func (s *authentikService) Update(ctx context.Context, slug string, req *goauthentik.ApplicationRequest) (*goauthentik.Application, error) {
	app, _, err := s.client.CoreApi.CoreApplicationsUpdate(ctx, slug).ApplicationRequest(*req).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, errors.Wrap(err, errUpdateApp)
	}
	return app, nil
}

func (s *authentikService) CreateOrUpdate(ctx context.Context, req *goauthentik.ApplicationRequest) (*goauthentik.Application, error) {
	app, err := s.Get(ctx, req.Slug)
	if err != nil {
		return nil, err
	}

	if app != nil {
		updated, err := s.Update(ctx, app.Slug, req)
		if err != nil {
			return nil, err
		}
		if updated != nil {
			return updated, nil
		}
	}

	return s.Create(ctx, req)
}

func (s *authentikService) Delete(ctx context.Context, slug string) error {
	_, err := s.client.CoreApi.CoreApplicationsDestroy(ctx, slug).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil
		}
		return errors.Wrap(err, errDeleteApp)
	}
	return nil
}
