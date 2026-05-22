# Getting Started

## Installation

### Prerequisites

To install this operator, you need the following:

- `kubectl`
- `kustomize` (Optional, required for the declarative kustomize method)
- `helm` (Optional, required for the Helm method)
- A kubernetes cluster with a recent enough version to support Custom Resource Definitions. The operator was initially built on `v1.22.5+k3s1` and being developed on `v1.25.4+k3s1`.

## Installation methods

### Helm (recommended)

Helm charts are published to `oci://ghcr.io/adyanth/charts` and provide the
simplest way to install and upgrade the operator.

#### 1. Install the operator

```bash
helm install cloudflare-operator oci://ghcr.io/adyanth/charts/cloudflare-operator \
  --namespace cloudflare-operator-system \
  --create-namespace
```

Key values (all optional — defaults shown):

| Value | Default | Description |
|---|---|---|
| `image.tag` | `0.13.1` | Operator image tag |
| `replicaCount` | `1` | Number of controller replicas |
| `namespace` | `cloudflare-operator-system` | Namespace to deploy into |
| `createNamespace` | `true` | Create the namespace if it does not exist |

#### 2. Create the Cloudflare API token Secret

Before creating tunnels, create a Kubernetes Secret with your Cloudflare API token
(see [operator-authentication](./examples/operator-authentication)):

```bash
kubectl create secret generic cloudflare-secrets \
  --namespace cloudflare-operator-system \
  --from-literal=cloudflare.apiToken=<YOUR_API_TOKEN>
```

#### 3. Create tunnels with the cloudflare-tunnels chart

The `cloudflare-tunnels` chart renders `ClusterTunnel` resources from a simple
values list, centralising shared connection settings across all your tunnels.

```bash
helm install cloudflare-tunnels oci://ghcr.io/adyanth/charts/cloudflare-tunnels \
  --namespace cloudflare-operator-system \
  --set cloudflare.email=you@example.com \
  --set cloudflare.accountId=<YOUR_ACCOUNT_ID> \
  --set cloudflare.secret=cloudflare-secrets \
  --set tunnels[0].name=my-tunnel \
  --set tunnels[0].domain=example.com
```

For multiple tunnels or persistent configuration, use a values file:

```yaml
# tunnels-values.yaml
cloudflare:
  email: you@example.com
  accountId: <YOUR_ACCOUNT_ID>
  secret: cloudflare-secrets

tunnels:
  - name: my-site
    domain: example.com
  - name: another-site
    domain: other.example.com
    replicas: 3          # override default of 2
    edgeIpVersion: "4"   # override default of auto
```

```bash
helm install cloudflare-tunnels oci://ghcr.io/adyanth/charts/cloudflare-tunnels \
  --namespace cloudflare-operator-system \
  -f tunnels-values.yaml
```

Key values for the `cloudflare-tunnels` chart:

| Value | Default | Description |
|---|---|---|
| `cloudflare.email` | `""` | **Required.** Cloudflare account email |
| `cloudflare.accountId` | `""` | **Required.** Cloudflare account ID |
| `cloudflare.secret` | `cloudflare-secrets` | Name of the API token Secret |
| `defaults.protocol` | `quic` | cloudflared protocol (`http2`, `h2mux`, `quic`, `auto`) |
| `defaults.edgeIpVersion` | `auto` | Edge IP version preference (`auto`, `4`, `6`) |
| `defaults.replicas` | `2` | Replicas per tunnel Deployment |
| `defaults.strategy.maxSurge` | `0` | Rolling update maxSurge |
| `defaults.strategy.maxUnavailable` | `1` | Rolling update maxUnavailable |

All `defaults.*` values can be overridden per-tunnel by setting the same key
under the tunnel entry in the `tunnels` list.

---

### Declarative installation with kustomize (GitOps)

1. Find the [latest tag for cloudflare-operator.](https://github.com/adyanth/cloudflare-operator/tags)
1. Create a kustomization.yaml in your repository that looks like
   ```yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   namespace: cloudflare-operator-system
   resources:
     # ensure you update the ref in this line to the latest version
     - https://github.com/adyanth/cloudflare-operator.git/config/default?ref=v0.13.1
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
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator.git//config/default?ref=v0.13.1'
```

#### Install the latest version

To install the latest version without checking tags, you can use either of the following.
This will deploy a point in time version of the operator.

```bash
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator.git/config/default?ref=main'
kubectl apply -k 'https://github.com/adyanth/cloudflare-operator/config/default'
```


## Where do I go from here?

Now that the operator is installed, we can make it useful.

1. [Deploy a secret with your API token](./examples/operator-authentication)
2. [Create a Tunnel/ClusterTunnel resource](./examples/tunnel-simple)
3. Configure routing for your tunnel by following one of:
    - [configure routing directly (simple)](./examples/tunnel-binding-simple)
    - [configure routing with a reverse proxy](./examples/tunnel-binding-with-reverse-proxy)

## Additional Info

* Look into the documentation in `docs/configuration` to understand various configurable parameters of this operator.
* Look at migration documentation in `docs/migrations` for upgrades.
