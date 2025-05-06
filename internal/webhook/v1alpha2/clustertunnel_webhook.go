/*
Copyright 2025 Adyanth H.

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

package v1alpha2

import (
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha2 "github.com/adyanth/cloudflare-operator/api/v1alpha2"
)

// nolint:unused
// log is for logging in this package.
var clustertunnellog = logf.Log.WithName("clustertunnel-resource")

// SetupClusterTunnelWebhookWithManager registers the webhook for ClusterTunnel in the manager.
func SetupClusterTunnelWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&networkingv1alpha2.ClusterTunnel{}).
		Complete()
}
