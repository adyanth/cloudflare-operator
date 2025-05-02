# Deploy a cloudflare tunnel

> This builds on [the getting started guide](../../../README.md#getting-started), and it is recommended to read that first.

This example shows how to deploy a cloudflare Tunnel/ClusterTunnel.
This resource will generate a cloudflared deployment for you.
You can optionally reference an existing cloudflared deployment, this document does not cover that use case. 

Once you have deployed this resource you can set up routing with a tunnelBinding.
See tunnel-binding examples for how to set up routing.

## Motivation

In order to route traffic from the internet to your cluster, a Tunnel/ClusterTunnel must be created.
This resource manages a deployment of one or more cloudflared pods which create an outbound connection to cloudflare's edge servers.
When this outbound connection is established, cloudflared can be enabled to route traffic from cloudflare domains to cloudflared.

Tunnels and ClusterTunnels are almost identical, a tunnel is simply a namespaced version of a clusterTunnel.
This is the same abstraction as is used for cert-manager's Issuer/ClusterIssuer resources.

The practical difference for you is that any tunnelBindings you set for a tunnel must be created in the same namespace as that tunnel.
A ClusterTunnel can be referenced by TunnelBindings from any namespace.

## Prerequisites

This assumes:
1. `kubectl` is installed
1. [You have deployed a secret to enable cloudflare-operator to authenticate to cloudflare](../operator-authentication)
1. [You have deployed cloudflare-operator](../operator-install)


## Steps

> All steps assume you are working from the directory this README is in.

> **Ensure that you have kubectl installed.**

1. Decide whether you want to deploy a Tunnel or ClusterTunnel.
   The configuration is the same, the only difference is the resource you configure. You will configure one (not both) of the following:
   - `manifests/tunnel.yaml`
   - `manifests/cluster-tunnel.yaml`
   
1. Replace all placeholder values formatted `<like-this>` in your Tunnel/ClusterTunnel manifest.
   - `<email-address>`: the email address associated with the cloudflare zone.
   - `<domain>`: the domain associated with the cloudflare zone you are managing.
   - `<secret-name>`: the name of the secret containing your cloudflare credentials.
   - `<account-id>`: the account ID associated with the cloudflare zone you are managing.

1. Deploy your Tunnel/ClusterTunnel
   ```bash
   # execute one of these, not both
   kubectl apply -f manifests/tunnel.yaml
   kubectl apply -f manifests/cluster-tunnel.yaml
   ```

1. Verify that the Tunnel/ClusterTunnel resource was successfully created and has generated a configmap and a deployment.

    ```bash
    kubectl get clustertunnel
    kubectl get tunnel -n cloudflare-operator-system
    # should return something like
    # NAME                 TUNNELID
    # k3s-cluster-tunnel   <uuid>
    ```

    ```bash
    kubectl get configmap k3s-cluster-tunnel -n cloudflare-operator-system
    # should return something like
    # NAME                                       DATA   AGE
    # k3s-cluster-tunnel                         1      5m
    ```

    ```bash
    kubectl get deployment k3s-cluster-tunnel -n cloudflare-operator-system
    # should return something like
    # NAME                                           READY   UP-TO-DATE   AVAILABLE   AGE
    # k3s-cluster-tunnel                             1/1     1            1           5m
    ```

At this point you can now make this Tunnel/ClusterTunnel useful by adding tunnelBindings.
- [Deploying a tunnelBinding (simple)](../tunnel-binding-simple)
- [Deploying a tunnelBinding (with reverse proxy)](../tunnel-binding-with-reverse-proxy)
