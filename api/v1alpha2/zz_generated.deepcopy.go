//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha2

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudflareDetails) DeepCopyInto(out *CloudflareDetails) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudflareDetails.
func (in *CloudflareDetails) DeepCopy() *CloudflareDetails {
	if in == nil {
		return nil
	}
	out := new(CloudflareDetails)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterTunnel) DeepCopyInto(out *ClusterTunnel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterTunnel.
func (in *ClusterTunnel) DeepCopy() *ClusterTunnel {
	if in == nil {
		return nil
	}
	out := new(ClusterTunnel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterTunnel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterTunnelList) DeepCopyInto(out *ClusterTunnelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterTunnel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterTunnelList.
func (in *ClusterTunnelList) DeepCopy() *ClusterTunnelList {
	if in == nil {
		return nil
	}
	out := new(ClusterTunnelList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterTunnelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExistingTunnel) DeepCopyInto(out *ExistingTunnel) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExistingTunnel.
func (in *ExistingTunnel) DeepCopy() *ExistingTunnel {
	if in == nil {
		return nil
	}
	out := new(ExistingTunnel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NewTunnel) DeepCopyInto(out *NewTunnel) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NewTunnel.
func (in *NewTunnel) DeepCopy() *NewTunnel {
	if in == nil {
		return nil
	}
	out := new(NewTunnel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Tunnel) DeepCopyInto(out *Tunnel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tunnel.
func (in *Tunnel) DeepCopy() *Tunnel {
	if in == nil {
		return nil
	}
	out := new(Tunnel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Tunnel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TunnelList) DeepCopyInto(out *TunnelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Tunnel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TunnelList.
func (in *TunnelList) DeepCopy() *TunnelList {
	if in == nil {
		return nil
	}
	out := new(TunnelList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TunnelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TunnelSpec) DeepCopyInto(out *TunnelSpec) {
	*out = *in
	out.Cloudflare = in.Cloudflare
	if in.ExistingTunnel != nil {
		in, out := &in.ExistingTunnel, &out.ExistingTunnel
		*out = new(ExistingTunnel)
		**out = **in
	}
	if in.NewTunnel != nil {
		in, out := &in.NewTunnel, &out.NewTunnel
		*out = new(NewTunnel)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TunnelSpec.
func (in *TunnelSpec) DeepCopy() *TunnelSpec {
	if in == nil {
		return nil
	}
	out := new(TunnelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TunnelStatus) DeepCopyInto(out *TunnelStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TunnelStatus.
func (in *TunnelStatus) DeepCopy() *TunnelStatus {
	if in == nil {
		return nil
	}
	out := new(TunnelStatus)
	in.DeepCopyInto(out)
	return out
}
