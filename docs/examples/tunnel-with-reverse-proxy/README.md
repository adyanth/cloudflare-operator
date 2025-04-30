# Bring your own reverse proxy

> This builds on [the getting started guide](../../getting-started.md), and it is recommended to read that first.

## Motivation

Cloudflare tunnels can be used as a reverse proxy, or can forward traffic to an existing reverse proxy.
This example shows how to configure cloudflared using the cloudflare-operator to front a reverse proxy. 
In effect, we are using it as a layer 7 or layer 4 load balancer depending on whether we forward TCP/UDP or HTTP/HTTPS traffic.

This configuration has a few use cases:
- You have significant existing reverse proxy configuration, which you may or may not want to migrate to cloudflared.
  This configuration allows additional routes to be added with a tunnel binding to use this reverse proxy in combination with other routes.
- You require reverse proxy functionality that cloudflared does not have
- You want to keep your reverse proxy configuration modular, so that you can use any load balancer implementation
- you use a service mesh with its own ingress implementation

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

## Steps

> All steps assume you are working from the directory this README is in.

> **Ensure that you have kubectl, kustomize, and helm installed.**

1. Configure a secret with your API token or API key, as described in [the getting started guide](../../getting-started.md#cloudflare-tokens)
2. Deploy the cloudflare operator
```shell
 kustomize build ../../../config/default | kubectl apply -f -
```
3. Replace all placeholder values formatted `<like-this>`.
   - in `manifests/cloudflare-operator`
      - `<email-address>`: the email address associated with the cloudflare zone.
      - `<domain>`: the domain associated with the cloudflare zone you are managing.
      - `<secret-name>`: the name of the secret containing your cloudflare credentials.
      - `<account-id>`: the account ID associated with the cloudflare zone you are managing.
   - in `manifests/hello/ingress.yaml`
     - `<domain>`: the domain associated with the cloudflare zone you are managing.
4. deploy the cluster tunnel and tunnelbinding
```shell
kubectl apply -f manifests/cloudflare-operator/
```
5. deploy the reverse proxy
```shell
kustomize build --enable-helm manifests/ingress-nginx | kubectl apply -f - 
```
6. deploy the example application
```shell
kubectl apply -f manifests/hello/
```
7. You should now be able to access the service on `hello.<domain>`

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
    # in order to direct cloudflare tunnel to ingress-nginx, we have a single tunnelBinding resource in ingress-nginx.
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
subjects:
  - name: authelia # maps to the fqdn authelia.<domain>
tunnelRef:
  kind: ClusterTunnel
  name: example-tunnel
---
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: TunnelBinding
metadata:
  name: ingress-nginx
subjects:
  - name: wildcard
    spec:
      fqdn: "*.<domain>"
      target: https://ingress-nginx-controller.ingress-nginx.svc.cluster.local:443
tunnelRef:
  kind: ClusterTunnel
  name: example-tunnel
```
