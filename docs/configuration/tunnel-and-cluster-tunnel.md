
# Tunnel and ClusterTunnel configuration

The Tunnel and the ClusterTunnel have the exact same configuration options. The best way to get the latest documentation on them is to run the below command after installing the CRDs.

```bash
kubectl explain tunnel.spec
```

Here is an overview of a ClusterTunnel as YAML for `v1alpha2`. Also look at migration docs for [`v1alpha1`](../migrations/crd/v1alpha2.md)

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha2
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
  noTlsVerify: false                        # Disables the TLS verification to backend services globally
  originCaPool: homelab-ca                  # Secret containing CA certificates to trust. Must contain tls.crt to be trusted globally and optionally other certificates (see the caPool service annotation for usage)
  deployPatch: |
    spec:
      replicas: 2
      template:
        spec:
          containers:
            - name: cloudflared
              image: cloudflare/cloudflared:2025.4.0 # Image to run. Used for running a pinned image. Can be swapped out to an arm based image if needed
```
