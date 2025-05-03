# Using Cloudflared as the reverse proxy

> This builds on [the getting started guide](../../install.md), and it is recommended to read that first.

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

## Steps

> All steps assume you are working from the directory this README is in.

> **Ensure that you have kubectl installed.**

To create a ClusterTunnel, we need to store Cloudflare credentials in a Secret. Follow the steps below. Have a look at the [detailed configuration guide](../../configuration.md) for more details.

1. Configure a secret with your API token or API key, as described in [the getting started guide](../../install.md#cloudflare-tokens)
2. Deploy the cloudflare operator as described in [the getting started guide](../../install.md#deploy-the-operator)

3. Deploy the example applications. This will create a deployment and service for each app.
    ```shell
    kubectl apply -f manifests/whoami-1/
    kubectl apply -f manifests/whoami-2/
    ```

3. After substituting values in `manifests/cloudflare-operator/cluster-tunnel.yaml`, deploy the ClusterTunnel.
    * The `newTunnel.name` is the name that shows up under Access > Tunnels on [Cloudflare For Teams Dashboard](https://dash.teams.cloudflare.com/)
    * The `cloudflare.email` is the email used to login to the dashboard. This is needed when using the Cloudflare Global API Key
    * The `cloudflare.domain` is the domain added and managed in Cloudflare under which new records are created
    * The `cloudflare.secret` is a reference to the Secret containing API keys and tokens. It should be in the same namespace as the Tunnel resource
    * The `accountId` is the one visible in the URL bar after logging into the [Cloudflare Dashboard](https://dash.cloudflare.com/). You can alternatively use `accountName` which is shown on the left panel once logged in.

    ```shell
    kubectl apply -f manifests/cloudflare-operator/clustertunnel.yaml
    ```

4. Verify that the ClusterTunnel resource was successfully created and has generated a configmap and a deployment.

    ```bash
    kubectl get clustertunnel
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

5. Deploy the tunnelBinding
    ```bash
    kubectl apply -f manifests/cloudflare-operator/tunnel-binding.yaml
    ```


6. Verify connectivity

    The name of the service and the domain of the ClusterTunnel is used to add the DNS record. 
    In this case, `whoami-1.example.com` and `whoami-2.example.com` would be added.
    
    You should now be able to navigate to these domains.
