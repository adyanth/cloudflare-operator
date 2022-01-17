# Getting Started

## Cloudflare Tokens

For the operator to interact with the [Cloudflare API](https://api.cloudflare.com/), an API Token is needed. To clean up and delete the DNS entries and the created tunnels, the Global API Key is needed.

These API tokens can be found under [My Profile > API Tokens](https://dash.cloudflare.com/profile/api-tokens) page in the Cloudflare Dashboard.

For the `CLOUDFLARE_API_KEY`, copy the Global API Key shown at the bottom of the page. This is used to delete the DNS entries and tunnels when the Service and Tunnel resources are deleted.

For the `CLOUDFLARE_API_TOKEN`, create a new "custom" token with the following:
1. Permissions
    * Account > Argo Tunnel > Edit : To create new tunnels
    * Account > Account Settings > Read : To get the accountId from Name and the domainId for the selected domain
    * Zone > DNS > Edit : To get the existing domain and create new entries in DNS for the domain. See #5 for potential unintended consequences if not careful when creating Resources.
2. Account Resources: Include > All accounts
3. Zone Resources: Include > All zones

Usage of these tokens can be validated from the source code of [cloudflare_api.go](../controllers/cloudflare_api.go).

## Prerequisites

To install this operator, you need the following:
* A kubernetes cluster with a recent enough version to support Custom Resource Definitions. The operator was initially built on `v1.22.5+k3s1`
* [`kubectl` cli](https://kubernetes.io/docs/tasks/tools/#kubectl) for deploying the operator, custom resources and samples.

## Deploy the Operator

Deploy the operator using Kustomize:

```bash
kubectl apply -k https://github.com/adyanth/cloudflare-operator/config/default
```

## Create a Custom Tunnel Resource

To create a Tunnel, we need to store Cloudflare credentials in a Secret. Follow the steps below.

1. Create a Secret containing Cloudflare credentials. More information on what these tokens do are [provided here](#cloudflare-tokens).
    
    ```bash
    kubectl create secret generic cloudflare-secrets --from-literal CLOUDFLARE_API_TOKEN=<api-token> --from-literal CLOUDFLARE_API_KEY=<api-key>
    ```

2. Create a Tunnel Resource using `kubectl apply -f tunnel.yaml`.
    * The `newTunnel.name` is the name that shows up under Access > Tunnels on [Cloudflare For Teams Dashboard](https://dash.teams.cloudflare.com/)
    * The `cloudflare.email` is the email used to login to the dashboard. This is needed when using the Cloudflare Global API Key
    * The `cloudflare.domain` is the domain added and managed in Cloudflare under which new records are created
    * The `cloudflare.secret` is a reference to the Secret containing API keys and tokens. It should be in the same namespace as the Tunnel resource
    * The `accountId` is the one visible in the URL bar after logging into the [Cloudflare Dashboard](https://dash.cloudflare.com/). You can alternatively use `accountName` which is shown on the left panel once logged in.

    ```yaml
    # tunnel.yaml
    apiVersion: networking.cfargotunnel.com/v1alpha1
    kind: Tunnel
    metadata:
        name: new-tunnel                # The Tunnel Custom Resource Name
    spec:
        newTunnel:
            name: my-k8s-tunnel         # Name of your new tunnel
        size: 2                         # This is the number of replicas for cloudflared
        cloudflare:
            email: email@domain.com     # Your email used to login to the Cloudflare Dashboard
            domain: example.com         # Domain under which the tunnel runs and adds DNS entries to
            secret: cloudflare-secrets  # The secret created before
            # accountId and accountName cannot be both empty. If both are provided, Account ID is used if valid, else falls back to Account Name.
            accountName: <Cloudflare account name>
            accountId: <Cloudflare account ID>
    ```

3. Verify that the tunnel resource was successful and generated a configmap and a deployment.

    ```bash
    kubectl get tunnel new-tunnel
    kubectl get configmap new-tunnel
    kubectl get deployment new-tunnel
    ```

## Sample Deployment and a Service to utilize the Tunnel

Deploy the below file using `kubectl apply -f sample.yaml` to run a [`whoami`](https://github.com/traefik/whoami) app and expose it to the internet using Cloudflare Tunnel.

The name of the service and the domain of the Tunnel is used to add the DNS record. In this case, `whoami.example.com` would be added.

```yaml
# sample.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
    name: whoami
    namespace: testing-crd
spec:
    selector:
        matchLabels:
            app: whoami
    template:
        metadata:
            labels:
                app: whoami
        spec:
            containers:
                - name: whoami
                  image: traefik/whoami
                  resources:
                      limits:
                          memory: "128Mi"
                          cpu: "500m"
                  ports:
                      - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
    name: whoami
    annotations:
        # Specifies the name of the Tunnel resource created before
        tunnels.networking.cfargotunnel.com/cr: new-tunnel
spec:
    selector:
        app: whoami
    ports:
        - port: 80
          targetPort: 80
```

And that's it. Head over to `whoami.example.com` to see your deployment exposed securely through Cloudflare Tunnels!
