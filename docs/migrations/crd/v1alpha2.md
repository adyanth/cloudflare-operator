# Migration guidance for CRD

[PR #145](https://github.com/adyanth/cloudflare-operator/pull/145) (operator release 0.13.0) introduced `v1alpha2` for Tunnel and ClusterTunnel resources.

This change removes the following fields in the `Cluster/Tunnel.Spec`:
* `image`
* `size`
* `nodeSelectors`
* `tolerations`

In place of this, a new field `deployPatch` is introduced. This allows any [kubectl style json/yaml patches](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_patch/) to be applied to the `cloudflared` deployment making any changes needed by the user possible without a CRD update.

Exising `v1alpha1` CRDs are automatically migrated to `v1alpha2` which is now the default storage version. Following are the example patches you would use for the fields above if you did not have them before the migration and now want to set them.

```yaml
apiVersion: networking.cfargotunnel.com/v1alpha2
kind: ClusterTunnel
...
spec:
    deployPatch: |
        spec:
          replicas: 2 # instead of size
            template:
              spec:
                tolerations: ... # tolerations as before
                nodeSelector: ... # nodeSelectors as before. Note the plural v/s singular
                containers:
                  - name: cloudflared # Note the usage of cloudflared name to select the container to patch
                    image: ... # image as before
...
```
