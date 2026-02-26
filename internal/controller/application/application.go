/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package application

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/pkg/errors"
	goauthentik "goauthentik.io/api/v3"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/crossplane/provider-authentik/apis/core/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-authentik/apis/v1alpha1"
)

const (
	errNotApplication = "managed resource is not a Application custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCPC         = "cannot get ClusterProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create new Service"

	errObserveApp  = "cannot observe Application"
	errCreateApp   = "cannot create Application"
	errUpdateApp   = "cannot update Application"
	errDeleteApp   = "cannot delete Application"
	errAppNotFound = "Application not found"
)

func stringPtrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func nullableStringEq(a goauthentik.NullableString, b *string) bool {
	aVal := a.Get()
	if aVal == nil && b == nil {
		return true
	}
	if aVal == nil || b == nil {
		return false
	}
	return *aVal == *b
}

func boolPtrEq(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// AuthentikService wraps the authentik API client
type AuthentikService struct {
	client *goauthentik.APIClient
}

func newAuthentikService(endpoint string, creds []byte) (interface{}, error) {
	token := string(creds)

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

	return &AuthentikService{
		client: goauthentik.NewAPIClient(cfg),
	}, nil
}

// SetupGated adds a controller that reconciles Application managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	// o.Gate.Register(func() {
	// 	if err := Setup(mgr, o); err != nil {
	// 		panic(errors.Wrap(err, "cannot setup Application controller"))
	// 	}
	// }, v1alpha1.ApplicationGroupVersionKind)
	return Setup(mgr, o)
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ApplicationGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newAuthentikService,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ApplicationList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.ApplicationList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.ApplicationGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Application{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        *resource.ProviderConfigUsageTracker
	newServiceFn func(endpoint string, creds []byte) (interface{}, error) // ← +endpoint
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return nil, errors.New(errNotApplication)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	var cd apisv1alpha1.ProviderCredentials
	var endpoint string

	ref := cr.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetPC)
		}
		endpoint = pc.Spec.Endpoint
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		endpoint = cpc.Spec.Endpoint
		cd = cpc.Spec.Credentials
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(endpoint, data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc.(*AuthentikService)}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service *AuthentikService
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotApplication)
	}

	fmt.Printf("Observing Application: %s\n", cr.Spec.ForProvider.Slug)

	if cr.GetDeletionTimestamp() != nil {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	apps, _, err := c.service.client.CoreApi.CoreApplicationsList(ctx).Slug(cr.Spec.ForProvider.Slug).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveApp)
	}

	if len(apps.Results) == 0 {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	app := apps.Results[0]

	cr.Status.SetConditions(xpv1.Available())
	cr.Status.AtProvider = v1alpha1.ApplicationObservation{
		ID: app.Pk,
	}

	upToDate := app.Name == cr.Spec.ForProvider.Name &&
		app.Slug == cr.Spec.ForProvider.Slug &&
		stringPtrEq(app.MetaDescription, cr.Spec.ForProvider.Description) &&
		stringPtrEq(app.MetaPublisher, cr.Spec.ForProvider.Publisher) &&
		stringPtrEq(app.Group, cr.Spec.ForProvider.Group) &&
		nullableStringEq(app.LaunchUrl, cr.Spec.ForProvider.LaunchUrl) &&
		stringPtrEq(app.MetaIcon, cr.Spec.ForProvider.IconUrl) &&
		boolPtrEq(app.OpenInNewTab, cr.Spec.ForProvider.OpenInNewTab)

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  upToDate,
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotApplication)
	}

	fmt.Printf("Creating Application: %s\n", cr.Spec.ForProvider.Slug)

	//convert int32 to SDK NullableInt32 type
	var provider goauthentik.NullableInt32
	if cr.Spec.ForProvider.Provider != nil {
		provider = *goauthentik.NewNullableInt32(cr.Spec.ForProvider.Provider)
	}

	app := goauthentik.ApplicationRequest{
		Name:                 cr.Spec.ForProvider.Name,
		Slug:                 cr.Spec.ForProvider.Slug,
		Provider:             provider,
		BackchannelProviders: cr.Spec.ForProvider.BackchannelProviders,
		OpenInNewTab:         cr.Spec.ForProvider.OpenInNewTab,
		MetaLaunchUrl:        cr.Spec.ForProvider.LaunchUrl,
		MetaIcon:             cr.Spec.ForProvider.IconUrl,
		MetaDescription:      cr.Spec.ForProvider.Description,
		MetaPublisher:        cr.Spec.ForProvider.Publisher,
		Group:                cr.Spec.ForProvider.Group,
	}

	_, _, err := c.service.client.CoreApi.CoreApplicationsCreate(ctx).ApplicationRequest(app).Execute()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateApp)
	}

	cr.Status.SetConditions(xpv1.Creating())

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotApplication)
	}

	fmt.Printf("Updating Application: %s\n", cr.Spec.ForProvider.Slug)

	apps, _, err := c.service.client.CoreApi.CoreApplicationsList(ctx).Slug(cr.Spec.ForProvider.Slug).Execute()
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateApp)
	}

	if len(apps.Results) == 0 {
		return managed.ExternalUpdate{}, errors.New(errAppNotFound)
	}

	app := apps.Results[0]

	var provider goauthentik.NullableInt32
	if cr.Spec.ForProvider.Provider != nil {
		provider = *goauthentik.NewNullableInt32(cr.Spec.ForProvider.Provider)
	}

	updateReq := goauthentik.ApplicationRequest{
		Name:                 cr.Spec.ForProvider.Name,
		Slug:                 cr.Spec.ForProvider.Slug,
		Provider:             provider,
		BackchannelProviders: cr.Spec.ForProvider.BackchannelProviders,
		OpenInNewTab:         cr.Spec.ForProvider.OpenInNewTab,
		MetaLaunchUrl:        cr.Spec.ForProvider.LaunchUrl,
		MetaIcon:             cr.Spec.ForProvider.IconUrl,
		MetaDescription:      cr.Spec.ForProvider.Description,
		MetaPublisher:        cr.Spec.ForProvider.Publisher,
		Group:                cr.Spec.ForProvider.Group,
	}

	_, _, err = c.service.client.CoreApi.CoreApplicationsUpdate(ctx, app.Slug).ApplicationRequest(updateReq).Execute()
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateApp)
	}

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotApplication)
	}

	fmt.Printf("Deleting Application: %s\n", cr.Spec.ForProvider.Slug)

	apps, _, err := c.service.client.CoreApi.CoreApplicationsList(ctx).Slug(cr.Spec.ForProvider.Slug).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteApp)
	}

	if len(apps.Results) > 0 {
		app := apps.Results[0]
		_, err = c.service.client.CoreApi.CoreApplicationsDestroy(ctx, app.Slug).Execute()
		if err != nil && !strings.Contains(err.Error(), "404") {
			return managed.ExternalDelete{}, errors.Wrap(err, errDeleteApp)
		}
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
