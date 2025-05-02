# Using Cloudflared as the reverse proxy

> This builds on [the getting started guide](../../../README.md#getting-started), and it is recommended to read that first.

This example shows how to configure cloudflared to route to two (or more) applications you are hosting.

You can of course configure only one application, but two are shown in this example so that it can be generalised.

```
+-----------------------+
|    internet users     |
+-----------------------+
            |
            v
+-----------------------+
|   cloudflare tunnel   |
+-----------------------+
     |             |
     v             v
 +-------+     +-------+ 
 | app 1 |     | app 2 | 
 +-------+     +-------+ 
```

## Motivation

Cloudflared is designed to be able to work as a reverse proxy in its own right, routing directly to your applications.

This is the simplest configuration one can follow and does not need an additional reverse proxy to handle routing.
All routing is handled by TunnelBinding resources managed by this controller.

## Prerequisites

This assumes:
1. `kubectl` is installed
1. [You have deployed a secret to enable cloudflare-operator to authenticate to cloudflare](../operator-authentication)
1. [You have deployed cloudflare-operator](../operator-install)
1. [You have deployed a Tunnel/ClusterTunnel](../tunnel-simple)

## Steps

> All steps assume you are working from the directory this README is in.

To create a ClusterTunnel, we need to store Cloudflare credentials in a Secret. Follow the steps below. Have a look at the [detailed configuration guide](../../configuration.md) for more details.

1. Deploy the example applications. This will create a deployment and service for each app.
    ```shell
    kubectl apply -f manifests/whoami-1/
    kubectl apply -f manifests/whoami-2/
    ```

1. Deploy the tunnelBinding
    ```bash
    kubectl apply -f manifests/cloudflare-operator/tunnel-binding.yaml
    ```

1. Verify connectivity

    The name of the service and the domain of the ClusterTunnel is used to add the DNS record. 
    In this case, `whoami-1.example.com` and `whoami-2.example.com` would be added.
    
    You should now be able to navigate to these domains.
