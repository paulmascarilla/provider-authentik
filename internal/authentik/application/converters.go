package application

import (
	goauthentik "goauthentik.io/api/v3"

	v1alpha1 "github.com/crossplane/provider-authentik/apis/core/v1alpha1"
)

func StringPtrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func NullableStringEq(a goauthentik.NullableString, b *string) bool {
	aVal := a.Get()
	if aVal == nil && b == nil {
		return true
	}
	if aVal == nil || b == nil {
		return false
	}
	return *aVal == *b
}

func BoolPtrEq(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func GenerateCreateRequest(cr *v1alpha1.Application) *goauthentik.ApplicationRequest {
	var provider goauthentik.NullableInt32
	if cr.Spec.ForProvider.Provider != nil {
		provider = *goauthentik.NewNullableInt32(cr.Spec.ForProvider.Provider)
	}

	return &goauthentik.ApplicationRequest{
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
}

func IsUpToDate(app *goauthentik.Application, cr *v1alpha1.Application) bool {
	return app.Name == cr.Spec.ForProvider.Name &&
		app.Slug == cr.Spec.ForProvider.Slug &&
		StringPtrEq(app.MetaDescription, cr.Spec.ForProvider.Description) &&
		StringPtrEq(app.MetaPublisher, cr.Spec.ForProvider.Publisher) &&
		StringPtrEq(app.Group, cr.Spec.ForProvider.Group) &&
		NullableStringEq(app.LaunchUrl, cr.Spec.ForProvider.LaunchUrl) &&
		StringPtrEq(app.MetaIcon, cr.Spec.ForProvider.IconUrl) &&
		BoolPtrEq(app.OpenInNewTab, cr.Spec.ForProvider.OpenInNewTab)
}

func GenerateObservation(app *goauthentik.Application) v1alpha1.ApplicationObservation {
	return v1alpha1.ApplicationObservation{
		ID:     app.Pk,
		Status: app.Name,
	}
}
