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

package v1alpha1

import (
	"reflect"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// ApplicationParameters are the configurable fields of a Application.
type ApplicationParameters struct {
	Name                 string  `json:"name"`
	Slug                 string  `json:"slug"`
	Provider             *int32  `json:"provider,omitempty"`
	BackchannelProviders []int32 `json:"backchannelProviders,omitempty"`
	OpenInNewTab         *bool   `json:"openInNewTab,omitempty"`
	LaunchUrl            *string `json:"launchUrl,omitempty"`
	IconUrl              *string `json:"iconUrl,omitempty"`
	Description          *string `json:"description,omitempty"`
	Publisher            *string `json:"publisher,omitempty"`
	Group                *string `json:"group,omitempty"`
}

// ApplicationObservation are the observable fields of a Application.
type ApplicationObservation struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

// A ApplicationSpec defines the desired state of a Application.
type ApplicationSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ApplicationParameters `json:"forProvider"`
}

// A ApplicationStatus represents the observed state of a Application.
type ApplicationStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ApplicationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Application is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,authentik}
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// SetConditions sets the conditions of the Application's status.
func (a *Application) SetConditions(c ...xpv1.Condition) {
	a.Status.SetConditions(c...)
}

// GetCondition returns the condition of the given type from the Application's status.
func (a *Application) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return a.Status.GetCondition(ct)
}

// GetManagementPolicies returns the ManagementPolicies from the embedded ManagedResourceSpec.
func (a *Application) GetManagementPolicies() xpv1.ManagementPolicies {
	return a.Spec.ManagementPolicies
}

// SetManagementPolicies sets the ManagementPolicies on the embedded ManagedResourceSpec.
func (a *Application) SetManagementPolicies(p xpv1.ManagementPolicies) {
	a.Spec.ManagementPolicies = p
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

// GetItems returns the list of Applications in the ApplicationList.
func (al *ApplicationList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(al.Items))
	for i := range al.Items {
		items[i] = &al.Items[i]
	}
	return items
}

// Application type metadata.
var (
	ApplicationKind             = reflect.TypeOf(Application{}).Name()
	ApplicationGroupKind        = schema.GroupKind{Group: CrossplaneGroup, Kind: ApplicationKind}.String()
	ApplicationKindAPIVersion   = ApplicationKind + "." + SchemeGroupVersion.String()
	ApplicationGroupVersionKind = SchemeGroupVersion.WithKind(ApplicationKind)
)

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
