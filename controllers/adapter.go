package controllers

import (
	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TunnelAdapater implementation
type TunnelAdapter struct {
	Tunnel       *networkingv1alpha1.Tunnel
	DefaultImage string
}

func (o TunnelAdapter) GetObject() client.Object {
	return o.Tunnel
}

func (o TunnelAdapter) GetNamespace() string {
	return o.Tunnel.Namespace
}

func (o TunnelAdapter) GetDefaultImage() string {
	return o.DefaultImage
}

func (o TunnelAdapter) GetName() string {
	return o.Tunnel.Name
}

func (o TunnelAdapter) GetUID() types.UID {
	return o.Tunnel.UID
}

func (o TunnelAdapter) GetLabels() map[string]string {
	return o.Tunnel.Labels
}

func (o TunnelAdapter) SetLabels(in map[string]string) {
	o.Tunnel.Labels = in
}

func (o TunnelAdapter) GetAnnotations() map[string]string {
	return o.Tunnel.Annotations
}

func (o TunnelAdapter) SetAnnotations(in map[string]string) {
	o.Tunnel.Annotations = in
}

func (o TunnelAdapter) GetSpec() networkingv1alpha1.TunnelSpec {
	return o.Tunnel.Spec
}

func (o TunnelAdapter) GetStatus() networkingv1alpha1.TunnelStatus {
	return o.Tunnel.Status
}

func (o TunnelAdapter) SetStatus(in networkingv1alpha1.TunnelStatus) {
	o.Tunnel.Status = in
}

func (o TunnelAdapter) DeepCopyTunnel() Tunnel {
	return TunnelAdapter{
		o.Tunnel.DeepCopy(),
		o.DefaultImage,
	}
}

// ClusterTunnelAdapter implementation
type ClusterTunnelAdapter struct {
	Tunnel       *networkingv1alpha1.ClusterTunnel
	Namespace    string
	DefaultImage string
}

func (o ClusterTunnelAdapter) GetObject() client.Object {
	return o.Tunnel
}

func (o ClusterTunnelAdapter) GetNamespace() string {
	return o.Namespace
}

func (o ClusterTunnelAdapter) GetDefaultImage() string {
	return o.DefaultImage
}

func (o ClusterTunnelAdapter) GetName() string {
	return o.Tunnel.Name
}

func (o ClusterTunnelAdapter) GetUID() types.UID {
	return o.Tunnel.UID
}

func (o ClusterTunnelAdapter) GetLabels() map[string]string {
	return o.Tunnel.Labels
}

func (o ClusterTunnelAdapter) SetLabels(in map[string]string) {
	o.Tunnel.Labels = in
}

func (o ClusterTunnelAdapter) GetAnnotations() map[string]string {
	return o.Tunnel.Annotations
}

func (o ClusterTunnelAdapter) SetAnnotations(in map[string]string) {
	o.Tunnel.Annotations = in
}

func (o ClusterTunnelAdapter) GetSpec() networkingv1alpha1.TunnelSpec {
	return o.Tunnel.Spec
}

func (o ClusterTunnelAdapter) GetStatus() networkingv1alpha1.TunnelStatus {
	return o.Tunnel.Status
}

func (o ClusterTunnelAdapter) SetStatus(in networkingv1alpha1.TunnelStatus) {
	o.Tunnel.Status = in
}

func (o ClusterTunnelAdapter) DeepCopyTunnel() Tunnel {
	return ClusterTunnelAdapter{
		o.Tunnel.DeepCopy(),
		o.Namespace,
		o.DefaultImage,
	}
}
