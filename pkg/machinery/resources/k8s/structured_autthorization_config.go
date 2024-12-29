// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// StructuredAuthorizationConfigType is type of StructuredAuthorizationConfig resource.
const StructuredAuthorizationConfigType = resource.Type("StructuredAuthorizationConfigs.kubernetes.talos.dev")

// StructuredAuthorizationConfigID is a singleton resource ID for StructuredAuthorizationConfig.
const StructuredAuthorizationConfigID = resource.ID("structured-authorization-config")

// StructuredAuthorizationConfig represents configuration for kube-apiserver structured authorization.
type StructuredAuthorizationConfig = typed.Resource[StructuredAuthorizationConfigSpec, StructuredAuthorizationConfigExtension]

// StructuredAuthorizationConfigSpec is structured authorization configuration for kube-apiserver.
//
//gotagsrewrite:gen
type StructuredAuthorizationConfigSpec struct {
	Config map[string]any `yaml:"config" protobuf:"1"`
}

// NewStructuredAuthorizationConfig returns new StructuredAuthorizationConfig resource.
func NewStructuredAuthorizationConfig() *StructuredAuthorizationConfig {
	return typed.NewResource[StructuredAuthorizationConfigSpec, StructuredAuthorizationConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, StructuredAuthorizationConfigType, StructuredAuthorizationConfigID, resource.VersionUndefined),
		StructuredAuthorizationConfigSpec{})
}

// StructuredAuthorizationConfigExtension defines StructuredAuthorizationConfig resource definition.
type StructuredAuthorizationConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (StructuredAuthorizationConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StructuredAuthorizationConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StructuredAuthorizationConfigSpec](StructuredAuthorizationConfigType, &StructuredAuthorizationConfig{})
	if err != nil {
		panic(err)
	}
}
