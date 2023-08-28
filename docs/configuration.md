# Configuration

The operator's behaviour can be controlled with various types of configuration. They are listed in sections below.

## Operator

The operator iself accepts command line arguments to override some of the default behaviours. They are as follows.

| **Command line argument**      | **Type** | **Description**                                                                                            | **Default Value**          |   |
|--------------------------------|----------|------------------------------------------------------------------------------------------------------------|----------------------------|---|
| `--default-image`              | string   | The default cloudflared image to use for tunnels                                                           |                            |   |
| `--cluster-resource-namespace` | string   | The default namespace for cluster scoped resources                                                         | cloudflare-operator-system |   |
| `--overwrite-unmanaged-dns`    | boolean  | Overwrite existing DNS records that do not have a corresponding managed TXT record                         | false                      |   |
| `--leader-elect`               | boolean  | Enable leader election for controller manager, this is optional for operator running with a single replica | true                       |   |

## Custom Resource Definition

### Tunnel and ClusterTunnel 

The Tunnel and the ClusterTunnel have the exact same configuration options. The best way to get the latest documentation on them is to run the below command after installing the CRDs.

```bash
kubectl explain tunnel.spec
```

Here is an overview of a ClusterTunnel as YAML.

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha1
kind: ClusterTunnel     # or Tunnel
metadata:
  name: tunnel-cr-name
spec:
  # Cloudflare details
  cloudflare:
    ## AccountName and AccountId cannot be both empty. If both are provided, Account ID is used if valid, else falls back to Account Name
    accountId: account-id
    accountName: Account Name
    domain: example.com                                                         # Domain where the tunnel runs
    email: admin@example.com                                                    # Email ID used to login to Cloudflare
    # Cloudflare credentials secret, and its key overrides. All the overrides are optional and default to the shown values.
    secret: cloudflare-secrets
    ## Key in the secret to use for Cloudflare API token. See getting started for information on scopes
    CLOUDFLARE_API_TOKEN: CLOUDFLARE_API_TOKEN
    ## Key in the secret to use for Cloudflare API Key. Needs Email also to be provided. For delete operations on new tunnels only, or as an alternate to API Token
    CLOUDFLARE_API_KEY: CLOUDFLARE_API_KEY
    ## Key in the secret to use as credentials.json for an existing tunnel
    CLOUDFLARE_TUNNEL_CREDENTIAL_FILE: CLOUDFLARE_TUNNEL_CREDENTIAL_FILE
    ## Key in the secret to use as tunnel secret for an existing tunnel
    CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET: CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET

  # Either existingTunnel or newTunnel can be specified, not both
  newTunnel:
    name: new-tunnel
  existingTunnel:
    ## Existing Tunnel id/name to run on. Tunnel Name and Tunnel ID cannot be both empty. If both are provided, id is used if valid, else falls back to name
    id: <tunnel-id>
    name: existing-tunnel

  # cloudflared configuration
  fallbackTarget: http_status:404           # The default service to point cloudflared to. Defaults to http_status:404
  podSpec:                                  # Overwrite any fields of the generated pod spec for cloudflared
    containers:
      - image: cloudflared:latest           # Overwrite image to be used
  noTlsVerify: false                        # Disables the TLS verification to backend services globally
  originCaPool: homelab-ca                  # Secret containing CA certificates to trust. Must contain tls.crt to be trusted globally and optionally other certificates (see the caPool service annotation for usage)
  size: 1                                   # Replica count for the tunnel deployment
```

### TunnelBinding

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

## Migrating from pre v0.9

Pre v0.9.x versions utilized service annotations with a service controller instead of the TunnelBinding resource. All the annotations neatly map to the custom resource definitions, and multiple services on the same tunnel can be mapped using a single TunnelBinding custom resource. The previous configuration options (which do not work anymore) are kept below for posterity.

### Service Configuration (deprecated)

Making a tunnel proxy a service is done through the TunnelBinding custom resource. Here are the available annotations. Only the first one is mandatory. Rest of them have defaults as needed.

* `cfargotunnel.com/tunnel` or `cfargotunnel.com/cluster-tunnel`: This annotation is needed for the Service controller to pick this service. Specify the name of the Tunnel/ClusterTunnel CRD which should serve this service
* `cfargotunnel.com/fqdn`: DNS name to access this service from. Defaults to the `service.metadata.name` + `tunnel.spec.domain`. If specifying this, make sure to use the same domain that the tunnel belongs to. This is not validated and used as provided
* `cfargotunnel.com/proto`: Specify the protocol for the service. Should be one of `http`, `https`, `tcp`, `udp`, `ssh` or `rdp`. Defaults to `http`, with the exceptions of `https` for 443, `smb` for 139 and 445, `rdp` for 3389 and `ssh` for 22 if the service has a TCP port. The only available option for a UDP port is `udp`, which is default
* `cfargotunnel.com/target`: Where the tunnel should proxy to. Defaults to the form of `<protocol>://<service.metadata.name>.<service.metadata.namespace>.svc:<port>`
* `cfargotunnel.com/caPool`: Specify the name of the key in the secret specified in `tunnel.spec.originCaPool` and that CA certificate will be trusted. `tls.crt` is trusted globally and does not need to be specified. Only useful if the protocol is HTTPS
* `cfargotunnel.com/noTlsVerify`: Disable TLS verification for this service. Only useful if the protocol is HTTPS
