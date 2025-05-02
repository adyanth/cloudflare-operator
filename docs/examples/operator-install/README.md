# Deploy cloudflare-operator

This example shows how to deploy a cloudflare Tunnel/ClusterTunnel.
Once you have deployed this resource you can set up routing with a tunnelBinding.
See tunnel-binding examples for how to set up routing.

## Motivation

## Prerequisites

To install this operator, you need the following:

- `kubectl`
- `kustomize` (Optional)
- A kubernetes cluster with a recent enough version to support Custom Resource Definitions. The operator was initially built on `v1.22.5+k3s1` and being developed on `v1.25.4+k3s1`.

## Installation methods

### Declarative installation (recommended)

1. Find the [latest tag for cloudflare-operator.](https://github.com/adyanth/cloudflare-operator/tags)
1. Create a kustomization.yaml in your repository that looks like
   ```yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   namespace: cloudflare-operator-system
   resources:
     # ensure you update the ref in this line to the latest version
     - https://github.com/adyanth/cloudflare-operator.git/config/default?ref=v0.11.1
   ```

1. deploy the application from the directory you placed the kustomization.yaml in
   ```bash
   # either approach will work
   kubectl apply -k .
   kustomize build . | kubectl apply -f -
   ```

If you need to customize the operator in some way, [you can do so with kustomize](https://glasskube.dev/blog/patching-with-kustomize/)

### Imperative installation

For a one-off installation, you can use any of the following methods

#### Install a specific tag

In general, one should pick a specific tag. 
[You can find the latest tag here](https://github.com/adyanth/cloudflare-operator/tags)

```bash
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator.git//config/default?ref=v0.12.0'
```

#### Install the latest version

To install the latest version without checking tags, you can use either of the following.
This will deploy a point in time version of the operator.

```bash
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator.git/config/default?ref=main'
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator/config/default'
```
