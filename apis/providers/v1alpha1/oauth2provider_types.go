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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// OAuth2ProviderParameters are the configurable fields of a OAuth2Provider.
type OAuth2ProviderParameters struct {
	ConfigurableField string `json:"configurableField"`
}

// OAuth2ProviderObservation are the observable fields of a OAuth2Provider.
type OAuth2ProviderObservation struct {
	ConfigurableField string `json:"configurableField"`
	ObservableField   string `json:"observableField,omitempty"`
}

// A OAuth2ProviderSpec defines the desired state of a OAuth2Provider.
type OAuth2ProviderSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              OAuth2ProviderParameters `json:"forProvider"`
}

// A OAuth2ProviderStatus represents the observed state of a OAuth2Provider.
type OAuth2ProviderStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          OAuth2ProviderObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A OAuth2Provider is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,authentik}
type OAuth2Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OAuth2ProviderSpec   `json:"spec"`
	Status OAuth2ProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OAuth2ProviderList contains a list of OAuth2Provider
type OAuth2ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OAuth2Provider `json:"items"`
}

// OAuth2Provider type metadata.
var (
	OAuth2ProviderKind             = reflect.TypeOf(OAuth2Provider{}).Name()
	OAuth2ProviderGroupKind        = schema.GroupKind{Group: Group, Kind: OAuth2ProviderKind}.String()
	OAuth2ProviderKindAPIVersion   = OAuth2ProviderKind + "." + SchemeGroupVersion.String()
	OAuth2ProviderGroupVersionKind = SchemeGroupVersion.WithKind(OAuth2ProviderKind)
)

func init() {
	SchemeBuilder.Register(&OAuth2Provider{}, &OAuth2ProviderList{})
}
