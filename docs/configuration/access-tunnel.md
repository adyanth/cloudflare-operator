# AccessTunnel
[Arbitrary TCP Access](https://developers.cloudflare.com/cloudflare-one/applications/non-http/cloudflared-authentication/arbitrary-tcp/) allows you to share a workload running on one cluster to be accessed by a service in another cluster that do not have local/tunnel connectivity between them. For example, a database running on cluster A exposed on port 5432 to be accessed by an application running on cluster B. The service to be published can be HTTP as well, but needs to be exposed as a TCP tunnel. This would be accomplished in two parts.

#### Expose the service as a tunnel
In the source cluster, expose the service using a TunnelBinding as before, using the protocol `tcp`. Cloudflare docs suggest the protocol of `rdp`, `smb`, `ssh` work as well, but these have not been tested. `udp` is supported in the tunnel, but the access side currently does not support it.

> **_NOTE:_**  The protocol needs to be TCP. Leaving it empty would use the default of HTTP, which will not work with Access.

> **_IMPORTANT:_**  Creating a tunnel like this makes it available on the internet for anyone to access. Make sure the application listening to the port is secure, or secure the application by using [Cloudflare Access ZTNA](https://developers.cloudflare.com/cloudflare-one/). If using ZTNA, make sure to supply the Service Token later below.

Ex: expose a postgres db on the source cluster

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: TunnelBinding
metadata:
  name: postgres
subjects:
  - kind: Service # Default
    name: postgres
    spec:
      fqdn: db.example.com
      protocol: tcp
tunnelRef:
  kind: Tunnel # Or ClusterTunnel
  name: k3s-tunnel
```

Now, on the client cluster, create an `AccessTunnel` object.

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: AccessTunnel
metadata:
  name: postgres
target:
  fqdn: db.example.com # From the tunnel
  protocol: tcp # Optional
  svc:
    port: 5432
# serviceToken: # Optional, needed if tunnel is secured using Cloudflare Access
#   secretRef: nameOfSecret
#   CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID: CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID # Optional to remap keys in secret
#   CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN: CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN # Optional to remap keys in secret
```

The client cluster should now be able to connect to `postgres.default.svc:5432` and be able to connect to the source cluster's DB.
