// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	schedulerv1 "k8s.io/kube-scheduler/config/v1"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// RenderConfigsStaticPodController manages k8s.ConfigsReady and renders configs for the control plane.
type RenderConfigsStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Name() string {
	return "k8s.RenderConfigsStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AdmissionControlConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AuditPolicyConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.StructuredAuthenticationConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.StructuredAuthorizationConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SchedulerConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.ConfigStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RenderConfigsStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		type configFile struct {
			filename string
			f        func() (runtime.Object, error)
		}

		admissionRes, err := safe.ReaderGetByID[*k8s.AdmissionControlConfig](ctx, r, k8s.AdmissionControlConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting admission config resource: %w", err)
		}

		admissionConfig := admissionRes.TypedSpec()

		auditRes, err := safe.ReaderGetByID[*k8s.AuditPolicyConfig](ctx, r, k8s.AuditPolicyConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting audit config resource: %w", err)
		}

		auditConfig := auditRes.TypedSpec()

		var structuredAuthNConfig *k8s.StructuredAuthenticationConfigSpec

		structuredAuthNRes, err := safe.ReaderGetByID[*k8s.StructuredAuthenticationConfig](ctx, r, k8s.StructuredAuthenticationConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting structured authentication config resource: %w", err)
		}

		structuredAuthNConfig = structuredAuthNRes.TypedSpec()

		var structuredAuthZConfig *k8s.StructuredAuthorizationConfigSpec

		structuredAuthZRes, err := safe.ReaderGetByID[*k8s.StructuredAuthorizationConfig](ctx, r, k8s.StructuredAuthorizationConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting structured authorization config resource: %w", err)
		}

		structuredAuthZConfig = structuredAuthZRes.TypedSpec()

		apiServerConfigs := make([]configFile, 0)

		if structuredAuthNConfig != nil && len(structuredAuthNConfig.Config) > 0 {
			apiServerConfigs = append(apiServerConfigs, configFile{
				filename: "authentication-config.yaml",
				f:        structuredAuthenticationConfig(structuredAuthNConfig),
			})
		}

		if structuredAuthZConfig != nil && len(structuredAuthZConfig.Config) > 0 {
			apiServerConfigs = append(apiServerConfigs, configFile{
				filename: "authorization-config.yaml",
				f:        structuredAuthorizationConfig(structuredAuthZConfig),
			})
		}

		kubeSchedulerRes, err := safe.ReaderGetByID[*k8s.SchedulerConfig](ctx, r, k8s.SchedulerConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting scheduler config resource: %w", err)
		}

		kubeSchedulerConfig := kubeSchedulerRes.TypedSpec()

		serializer := k8sjson.NewSerializerWithOptions(
			k8sjson.DefaultMetaFactory, nil, nil,
			k8sjson.SerializerOptions{
				Yaml:   true,
				Pretty: true,
				Strict: true,
			},
		)

		for _, pod := range []struct {
			name      string
			directory string
			uid       int
			gid       int
			configs   []configFile
		}{
			{
				name:      "kube-apiserver",
				directory: constants.KubernetesAPIServerConfigDir,
				uid:       constants.KubernetesAPIServerRunUser,
				gid:       constants.KubernetesAPIServerRunGroup,
				configs: append([]configFile{
					{
						filename: "admission-control-config.yaml",
						f:        admissionControlConfig(admissionConfig),
					},
					{
						filename: "auditpolicy.yaml",
						f:        auditPolicyConfig(auditConfig),
					},
				}, apiServerConfigs...),
			},
			{
				name:      "kube-scheduler",
				directory: constants.KubernetesSchedulerConfigDir,
				uid:       constants.KubernetesSchedulerRunUser,
				gid:       constants.KubernetesSchedulerRunGroup,
				configs: []configFile{
					{
						filename: "scheduler-config.yaml",
						f:        schedulerConfig(kubeSchedulerConfig),
					},
				},
			},
		} {
			if err = os.MkdirAll(pod.directory, 0o755); err != nil {
				return fmt.Errorf("error creating config directory for %q: %w", pod.name, err)
			}

			for _, configFile := range pod.configs {
				var obj runtime.Object

				obj, err = configFile.f()
				if err != nil {
					return fmt.Errorf("error generating configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				var buf bytes.Buffer

				if err = serializer.Encode(obj, &buf); err != nil {
					return fmt.Errorf("error marshaling configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				if err = os.WriteFile(filepath.Join(pod.directory, configFile.filename), buf.Bytes(), 0o400); err != nil {
					return fmt.Errorf("error writing configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				if err = os.Chown(filepath.Join(pod.directory, configFile.filename), pod.uid, pod.gid); err != nil {
					return fmt.Errorf("error chowning %q for %q: %w", configFile.filename, pod.name, err)
				}
			}
		}

		if err = safe.WriterModify(ctx, r, k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID), func(r *k8s.ConfigStatus) error {
			r.TypedSpec().Ready = true
			r.TypedSpec().Version = admissionRes.Metadata().Version().String() + auditRes.Metadata().Version().String() + kubeSchedulerRes.Metadata().Version().String()

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func admissionControlConfig(spec *k8s.AdmissionControlConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg apiserverv1.AdmissionConfiguration

		cfg.APIVersion = apiserverv1.SchemeGroupVersion.String()
		cfg.Kind = "AdmissionConfiguration"
		cfg.Plugins = []apiserverv1.AdmissionPluginConfiguration{}

		for _, plugin := range spec.Config {
			raw, err := json.Marshal(plugin.Configuration)
			if err != nil {
				return nil, fmt.Errorf("error marshaling configuration for plugin %q: %w", plugin.Name, err)
			}

			cfg.Plugins = append(cfg.Plugins,
				apiserverv1.AdmissionPluginConfiguration{
					Name: plugin.Name,
					Configuration: &runtime.Unknown{
						Raw: raw,
					},
				},
			)
		}

		return &cfg, nil
	}
}

func auditPolicyConfig(spec *k8s.AuditPolicyConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg auditv1.Policy

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(spec.Config, &cfg, true); err != nil {
			return nil, fmt.Errorf("error unmarshaling audit policy configuration: %w", err)
		}

		return &cfg, nil
	}
}

func structuredAuthenticationConfig(spec *k8s.StructuredAuthenticationConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg apiserverv1beta1.AuthenticationConfiguration

		fmt.Println(spec.Config)

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(spec.Config, &cfg, true); err != nil {
			return nil, fmt.Errorf("error unmarshaling structured authentication configuration: %w", err)
		}

		cfg.APIVersion = "apiserver.config.k8s.io/v1beta1"
		cfg.Kind = "AuthenticationConfiguration"

		return &cfg, nil
	}
}

func structuredAuthorizationConfig(spec *k8s.StructuredAuthorizationConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg apiserverv1beta1.AuthorizationConfiguration

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(spec.Config, &cfg, true); err != nil {
			return nil, fmt.Errorf("error unmarshaling structured authorization configuration: %w", err)
		}

		cfg.APIVersion = "apiserver.config.k8s.io/v1beta1"
		cfg.Kind = "AuthorizationConfiguration"

		return &cfg, nil
	}
}

func schedulerConfig(spec *k8s.SchedulerConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg schedulerv1.KubeSchedulerConfiguration

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(spec.Config, &cfg, false); err != nil {
			return nil, fmt.Errorf("error unmarshaling scheduler configuration: %w", err)
		}

		cfg.APIVersion = "kubescheduler.config.k8s.io/v1"
		cfg.Kind = "KubeSchedulerConfiguration"
		cfg.ClientConnection.Kubeconfig = filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig")

		return &cfg, nil
	}
}
