# Bring your own reverse proxy

> This builds on [the getting started guide](../../../README.md#getting-started), and it is recommended to read that first.

This example configures cloudflared to send all traffic to a reverse proxy, and then configure routing in the reverse proxy.

```
+----------------------+
|   internet users     |
+----------------------+
           |
           v
+----------------------+
|  cloudflare tunnel   |
+----------------------+
           |
           v
+----------------------+
|    reverse proxy     |
+----------------------+
     |           |
     v           v
+---------+ +----------+
| App #1  | |  App #2  |
+---------+ +----------+
```

## Motivation

Cloudflare tunnels can be used as a reverse proxy, or can forward traffic to an existing reverse proxy.
This example shows how to configure cloudflared using the cloudflare-operator to front a reverse proxy. 
In effect, we are using it as a layer 4 or 7 load balancer depending on whether we forward TCP/UDP or HTTP/HTTPS traffic respectively.

This configuration has a few use cases:
- You have significant existing reverse proxy configuration, which you may or may not want to migrate to cloudflared.
  This configuration allows additional routes to be added with a tunnel binding to use this reverse proxy in combination with other routes.
- You require reverse proxy functionality that cloudflared does not have
- You want to keep your reverse proxy configuration modular, so that you can use any load balancer implementation
- You use a service mesh with its own ingress implementation
- You have multiple load balancers pointing at your ingress. For example a lab might include metallb + DNS entries so that you are not traversing the internet when on the same network as your cluster

## What have we got here

In this example we will:
1. Deploy cloudflare-operator and configure a cluster tunnel (this would also work with a regular tunnel resource).
2. Deploy ingress-nginx as a reference reverse proxy, and configure the created cluster tunnel to point to ingress-nginx.
3. Deploy an example application, and configure it behind nginx with an ingress resource
   
Doing so will allow you to use the cloudflare operator as a shim to whatever ingress method you choose.
One can change the ingress implementation at will and use e.g. gateway API, a different ingress, etc

Notes:
- We use a cluster tunnel, but one can equally use a tunnel resource in the same way.
- We do not use https with the reverse proxy for simplicity. 
  The example deployment yaml includes commented snippets one can use to enable https.

```
.
├── README.md                   # this document
└── manifests                   # contains all manifests from this example
    ├── cloudflare-operator     # contains manifests for configuring cloudflare-operator
    │   ├── tunnel-binding.yaml    
    │   └── cluster-tunnel.yaml
    ├── hello                   # a reference application, you would replace this with your own application
    │   ├── deployment.yaml
    │   ├── ingress.yaml
    │   └── service.yaml    
    └── ingress-nginx           # this example deploys ingress-nginx as the reverse proxy
        ├── kustomization.yaml
        ├── namespace.yaml
        └── values.yaml
```

## Prerequisites

This assumes:
1. `kubectl` is installed
1. `kustomize` is installed
1. `helm` is installed
1. [You have deployed a secret to enable cloudflare-operator to authenticate to cloudflare](../operator-authentication)
1. [You have deployed cloudflare-operator](../operator-install)
1. [You have deployed a Tunnel/ClusterTunnel](../tunnel-simple)

## Steps

> All steps assume you are working from the directory this README is in.

1. Replace all placeholder values formatted `<like-this>`.
   - in `manifests/hello/ingress.yaml`
     - `<domain>`: the domain associated with the cloudflare zone you are managing.
       This should match your tunnel/clusterTunnel.
   - in `manifests/tunnel-binding.yaml`
       - `<domain>`: the domain associated with the cloudflare zone you are managing.
         This should match your tunnel/clusterTunnel.
1. deploy the TunnelBinding
   ```shell
   kubectl apply -f manifests/tunne-binding.yaml
   ```

1. deploy the reverse proxy
   ```shell
   kustomize build --enable-helm manifests/ingress-nginx | kubectl apply -f - 
   ```
1. deploy the example application
   ```shell
   kubectl apply -f manifests/hello/
   ```

1. You should now be able to access the service on `hello.<domain>`

## Extending this example

Now that we are using an arbitrary reverse proxy for routing traffic into the cluster, we can configure it as if we were using a cloud load balancer.
This means you can use whatever constructs are available to your reverse proxy implementation.

### Custom SSO

One can configure SSO with a custom provider.
This example uses ingress-nginx and authelia

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: <app>
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    # in order to direct cloudflare tunnel to ingress-nginx, we have a single TunnelBinding resource in ingress-nginx.
    # this is what controls ingress to ingress-nginx and allows us to put 2fa infront of apps that don't support it natively.
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/auth-method: "GET"
    nginx.ingress.kubernetes.io/auth-url: "http://authelia.authelia.svc.cluster.local:8080/api/authz/auth-request"
    nginx.ingress.kubernetes.io/auth-signin: "https://authelia.<domain>?rm=$request_method"
    nginx.ingress.kubernetes.io/auth-response-headers: "Remote-User,Remote-Name,Remote-Groups,Remote-Email"
--- SNIP ---
```

### Using cloudflared for routing as well as ingress-nginx

Using this approach does not prohibit additional routes.
This is useful if you are migrating to/from one approach, or have different requirements for different endpoints.
For example, you may want some endpoints to bypass your reverse proxy for some reason.

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: TunnelBinding
metadata:
  name: authelia
  namespace: authelia
subjects:
  - name: authelia # the fqdn authelia.<domain> maps to this service
tunnelRef:
  kind: ClusterTunnel
  name: example-tunnel
---
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: TunnelBinding
metadata:
  # At this time cloudflare-operator does not deterministically sort wildcards to the end of cloudflared's config file.
  # This is important because cloudflared goes through the list in order, and this ingress being before the authelia ingress
  # would mean authelia is impossible to reach.
  # By prefixing with `zz-` we ensure this is the last route in the cloudflared config file
  name: zz-ingress-nginx
subjects:
  - name: wildcard
    spec:
      fqdn: "*.<domain>"
      target: https://ingress-nginx-controller.ingress-nginx.svc.cluster.local:443
tunnelRef:
  kind: ClusterTunnel
  name: example-tunnel
```
