package controller

import (
	networkingv1alpha2 "github.com/adyanth/cloudflare-operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Tunnel interface {
	GetObject() client.Object
	GetNamespace() string
	GetName() string
	GetLabels() map[string]string
	SetLabels(map[string]string)
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)
	GetSpec() networkingv1alpha2.TunnelSpec
	GetStatus() networkingv1alpha2.TunnelStatus
	SetStatus(networkingv1alpha2.TunnelStatus)
	DeepCopyTunnel() Tunnel
}
