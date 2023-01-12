package controllers

import (
	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Tunnel interface {
	GetObject() client.Object
	GetNamespace() string
	GetName() string
	GetDefaultImage() string
	GetLabels() map[string]string
	SetLabels(map[string]string)
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)
	GetSpec() networkingv1alpha1.TunnelSpec
	GetStatus() networkingv1alpha1.TunnelStatus
	SetStatus(networkingv1alpha1.TunnelStatus)
	DeepCopyTunnel() Tunnel
}
