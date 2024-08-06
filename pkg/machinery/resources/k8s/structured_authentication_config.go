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

// StructuredAuthenticationConfigType is type of StructuredAuthenticationConfig resource.
const StructuredAuthenticationConfigType = resource.Type("StructuredAuthenticationConfigs.kubernetes.talos.dev")

// StructuredAuthenticationConfigID is a singleton resource ID for StructuredAuthenticationConfig.
const StructuredAuthenticationConfigID = resource.ID("structured-authentication-config")

// StructuredAuthenticationConfig represents configuration for kube-apiserver structured authentication.
type StructuredAuthenticationConfig = typed.Resource[StructuredAuthenticationConfigSpec, StructuredAuthenticationConfigExtension]

// StructuredAuthenticationConfigSpec is structured authentication configuration for kube-apiserver.
//
//gotagsrewrite:gen
type StructuredAuthenticationConfigSpec struct {
	Config map[string]any `yaml:"config" protobuf:"1"`
}

// NewStructuredAuthenticationConfig returns new StructuredAuthenticationConfig resource.
func NewStructuredAuthenticationConfig() *StructuredAuthenticationConfig {
	return typed.NewResource[StructuredAuthenticationConfigSpec, StructuredAuthenticationConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, StructuredAuthenticationConfigType, StructuredAuthenticationConfigID, resource.VersionUndefined),
		StructuredAuthenticationConfigSpec{})
}

// StructuredAuthenticationConfigExtension defines StructuredAuthenticationConfig resource definition.
type StructuredAuthenticationConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (StructuredAuthenticationConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StructuredAuthenticationConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StructuredAuthenticationConfigSpec](StructuredAuthenticationConfigType, &StructuredAuthenticationConfig{})
	if err != nil {
		panic(err)
	}
}
