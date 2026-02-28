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
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/crossplane/provider-authentik/apis/core/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-authentik/apis/v1alpha1"
	authentik "github.com/crossplane/provider-authentik/internal/authentik"
	"github.com/crossplane/provider-authentik/internal/authentik/application"
)

const (
	errNotApplication = "managed resource is not a Application custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCPC         = "cannot get ClusterProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create new Service"
)

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return Setup(mgr, o)
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ApplicationGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
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

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

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
			if cr.GetDeletionTimestamp() != nil {
				return &external{nil}, nil
			}
			return nil, errors.Wrap(err, errGetPC)
		}
		endpoint = pc.Spec.Endpoint
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			if cr.GetDeletionTimestamp() != nil {
				return &external{nil}, nil
			}
			return nil, errors.Wrap(err, errGetCPC)
		}
		endpoint = cpc.Spec.Endpoint
		cd = cpc.Spec.Credentials
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		if cr.GetDeletionTimestamp() != nil {
			return &external{nil}, nil
		}
		return nil, errors.Wrap(err, errGetCreds)
	}

	client, err := authentik.NewClientFromData(endpoint, data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	svc := application.NewService(client)

	return &external{service: svc}, nil
}

type external struct {
	service application.ApplicationService
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotApplication)
	}

	if c.service == nil {
		if cr.GetDeletionTimestamp() != nil {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.New("no authentik service available")
	}

	fmt.Printf("Observing Application: %s (DeletionTimestamp: %v)\n", cr.Spec.ForProvider.Slug, cr.GetDeletionTimestamp())

	if cr.GetDeletionTimestamp() != nil {
		app, err := c.service.Get(ctx, cr.Spec.ForProvider.Slug)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "cannot observe Application")
		}
		if app == nil {
			fmt.Printf("Application %s marked for deletion, external resource gone\n", cr.Spec.ForProvider.Slug)
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{
			ResourceExists: true,
		}, nil
	}

	app, err := c.service.Get(ctx, cr.Spec.ForProvider.Slug)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot observe Application")
	}

	if app == nil {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	cr.Status.SetConditions(xpv1.Available())
	cr.Status.AtProvider = application.GenerateObservation(app)

	upToDate := application.IsUpToDate(app, cr)

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

	req := application.GenerateCreateRequest(cr)

	_, err := c.service.Create(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create Application")
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

	req := application.GenerateCreateRequest(cr)

	_, err := c.service.CreateOrUpdate(ctx, req)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update Application")
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

	err := c.service.Delete(ctx, cr.Spec.ForProvider.Slug)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete Application")
	}

	fmt.Printf("Application %s deleted from authentik\n", cr.Spec.ForProvider.Slug)

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
