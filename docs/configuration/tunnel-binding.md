# TunnelBinding

This replaces the older implementation which used annotations on services to configure the endpoints. The TunnelBinding resource, inspired by RoleBinding, uses a similar structure with `subjects`, which are the target services to tunnel, and `tunnelRef` which provides details on what tunnel to use. Below is a detailed sample. Again, using `kubectl explain tunnelbinding.subjects` and `kubectl explain tunnelbinding.tunnelRef` gives the latest documentation on these. Below are the new config options over the service annotations.

* `tunnelRef.disableDNSUpdates`: Disables DNS record updates by the controller. You need to manually add the CNAME entries to point to the tunnel domain. The tunnel domain is of the form `tunnel-id.cfargotunnel.com`. The tunnel ID can be found using `kubectl get clustertunnel/tunnel <tunnel-name>`. You can also make use of the [proxied wildcard domains](https://blog.cloudflare.com/wildcard-proxy-for-everyone/) to CNAME `*.domain.com` to your tunnel domain so that manual DNS updates are not required.

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: TunnelBinding
metadata:
  name: svc-binding
subjects:
  - kind: Service # Default
    name: svc01
    spec:
      fqdn: mysvc.example.com
      protocol: http
      target: http://svc01.ns.svc.cluster.local:8080
      caPool: custom.crt
      noTlsVerify: false
  - name: svc02  # Points to the second service
tunnelRef:
  kind: Tunnel # Or ClusterTunnel
  name: k3s-tunnel
  disableDNSUpdates: false
```
