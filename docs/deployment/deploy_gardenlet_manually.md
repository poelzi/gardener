# Deploy a Gardenlet Manually

Manually deploying a gardenlet is required in the following cases:

- The Kubernetes cluster to be registered as a seed cluster has no public endpoint, 
  because it is behind a firewall. 
  The gardenlet must then be deployed into the cluster itself.

- The Kubernetes cluster to be registered as a seed cluster is managed externally 
  (the Kubernetes cluster is not a shoot cluster, so [Automatic Deployment of Gardenlets](deploy_gardenlet_automatically.md) cannot be used).

- The gardenlet runs outside of the Kubernetes cluster 
  that should be registered as a seed cluster. 
  (The gardenlet is not restricted to run in the seed cluster or 
  to be deployed into a Kubernetes cluster at all).

> Once you’ve deployed a gardenlet manually, for example, behind a firewall, you can deploy new gardenlets automatically. The manually deployed gardenlet is then used as a template for the new gardenlets. More information: [Automatic Deployment of Gardenlets](deploy_gardenlet_automatically.md).

## Prerequisites

### Kubernetes cluster that should be registered as a seed cluster
    
- Verify that the cluster has a [supported Kubernetes version](../usage/supported_k8s_versions.md).
  
- Determine the nodes, pods, and services CIDR of the cluster. 
  You need to configure this information in the `Seed` configuration.
  Gardener uses this information to check that the shoot cluster isn’t created with overlapping CIDR ranges.

- Every Seed cluster needs an Ingress controller which distributes external requests to internal components like grafana and prometheus. Gardener supports two approaches to achieve this:
    
a. Gardener managed Ingress controller and DNS records. For this configure the following lines in your [Seed resource](../../example/50-seed.yaml):
```yaml
spec:
  dns:
    provider:
      type: aws-route53
      secretRef:
        name: ingress-secret
        namespace: garden
  ingress:
    domain: ingress.my-seed.example.com
    controller:
      kind: nginx
      providerConfig:
        <some-optional-provider-specific-config-for-the-ingressController>
```

⚠ Please note that if you set `.spec.ingress` then `.spec.dns.ingressDomain` must be `nil`.

b. Self-managed DNS record and Ingress controller:

:warning:  
There should exist a DNS record `*.ingress.<SEED-CLUSTER-DOMAIN>` where `<SEED-CLUSTER-DOMAIN>` is the value of the `.dns.ingressDomain` field of [a Seed cluster resource](../../example/50-seed.yaml) (or the [respective Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L84-L85)).

*This is how it could be done for the Nginx ingress controller*

Deploy nginx into the `kube-system` namespace in the Kubernetes cluster that should be registered as a `Seed`.

Nginx will on most cloud providers create the service with type `LoadBalancer` with an external ip.

```
NAME                        TYPE           CLUSTER-IP    EXTERNAL-IP
nginx-ingress-controller    LoadBalancer   10.0.15.46    34.200.30.30
```

Create a wildcard `A` record (e.g *.ingress.sweet-seed.<my-domain>. IN A 34.200.30.30) with your DNS provider and point it to the external ip of the ingress service. This ingress domain is later required to register the `Seed` cluster.

Please configure the ingress domain in the `Seed` specification as follows:

```yaml
spec:
  dns:
    ingressDomain: ingress.sweet-seed.<my-domain>
```

⚠ Please note that if you set `.spec.dns.ingressDomain` then `.spec.ingress` must be `nil`.

### `kubeconfig` for the Seed Cluster

The `kubeconfig` is required to deploy the gardenlet Helm chart to the seed cluster. 
This deployment requires admin privileges. 
The Helm chart contains a service account `gardenlet` 
that the gardenlet deployment uses by default to talk to the Seed API server.

> If the gardenlet isn’t deployed in the seed cluster, 
> the gardenlet can be configured to use a `kubeconfig`, 
> which also requires full admin rights, from a mounted directory. 
> The `kubeconfig` is specified in section `seedClientConnection.kubeconfig` 
> of the [Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml).
> This configuration option isn’t used in  the following,  
> as this procedure only describes the recommended setup option 
> where the gardenlet is running in the seed cluster itself. 

## Procedure Overview

1. Prepare the garden cluster:
    1. [Create a bootstrap token secret in the `kube-system` namespace of the garden cluster](#create-a-bootstrap-token-secret-in-the-kube-system-namespace-of-the-garden-cluster)
    1. [Create RBAC roles for the gardenlet to allow bootstrapping in the garden cluster](#create-rbac-roles-for-the-gardenlet-to-allow-bootstrapping-in-the-garden-cluster)
   
1. [Prepare the gardenlet Helm chart](#prepare-the-gardenlet-helm-chart).
1. [Automatically register shoot cluster as a seed cluster](#automatically-register-shoot-cluster-as-a-seed-cluster).
1. [Deploy the gardenlet](#deploy-the-gardenlet)
1. [Check that the gardenlet is successfully deployed](#check-that-the-gardenlet-is-successfully-deployed)

## Create a bootstrap token secret in the `kube-system` namespace of the garden cluster

The gardenlet needs to talk to the [Gardener API server](../concepts/apiserver.md) residing in the garden cluster. 

The gardenlet can be configured with an already existing garden cluster `kubeconfig` in one of the following ways: 

  - Either by specifying `gardenClientConnection.kubeconfig` 
    in the [Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml) or

  - by supplying the environment variable `GARDEN_KUBECONFIG` pointing to 
    a mounted `kubeconfig` file).

The preferred way however, is to use the gardenlets ability to request 
a signed certificate for the garden cluster by leveraging 
[Kubernetes Certificate Signing Requests](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/).
The gardenlet performs a TLS bootstrapping process that is similar to the 
[Kubelet TLS Bootstrapping](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/).
Make sure that the API server of the garden cluster has 
[bootstrap token authentication](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/#enabling-bootstrap-token-authentication) 
enabled.

The client credentials required for the gardenlets TLS bootstrapping process, 
need to be either `token` or `certificate` (OIDC isn’t supported) and have permissions
to create a Certificate Signing Request ([CSR](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)).
It’s recommended to use [bootstrap tokens](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) 
due to their desirable security properties (such as a limited token lifetime).

Therefore, first create a bootstrap token secret for the garden cluster: 

``` yaml
apiVersion: v1
kind: Secret
metadata:
  # Name MUST be of form "bootstrap-token-<token id>"
  name: bootstrap-token-07401b
  namespace: kube-system

# Type MUST be 'bootstrap.kubernetes.io/token'
type: bootstrap.kubernetes.io/token
stringData:
  # Human readable description. Optional.
  description: "Token to be used by the gardenlet for Seed `sweet-seed`."

  # Token ID and secret. Required.
  token-id: 07401b # 6 characters
  token-secret: f395accd246ae52d # 16 characters

  # Expiration. Optional.
  # expiration: 2017-03-10T03:22:11Z

  # Allowed usages.
  usage-bootstrap-authentication: "true"
  usage-bootstrap-signing: "true"
```

When you later prepare the gardenlet Helm chart, 
a `kubeconfig` based on this token is shared with the gardenlet upon deployment.

## Create RBAC roles for the gardenlet to allow bootstrapping in the garden cluster
   
This step is only required if the gardenlet you deploy is the first gardenlet 
in the Gardener installation. 
Additionally, when using the [control plane chart](../../charts/gardener/controlplane), 
the following resources are already contained in the Helm chart, 
that is, if you use it you can skip these steps as the needed RBAC roles already exist. 

The gardenlet uses the configured bootstrap `kubeconfig` in `gardenClientConnection.bootstrapKubeconfig` to request a signed certificate for the user `gardener.cloud:system:seed:<seed-name>` in the group `gardener.cloud:system:seeds`.

Create a `ClusterRole` and `ClusterRoleBinding` that grant full admin permissions to authenticated gardenlets.  

Create the following resources in the garden cluster:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gardener.cloud:system:seeds
rules:
  - apiGroups:
      - '*'
    resources:
      - '*'
    verbs:
      - '*'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gardener.cloud:system:seeds
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:system:seeds
subjects:
  - kind: Group
    name: gardener.cloud:system:seeds
    apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gardener.cloud:system:seed-bootstrapper
rules:
  - apiGroups:
      - certificates.k8s.io
    resources:
      - certificatesigningrequests
    verbs:
      - create
      - get
      - list
      - watch
  - apiGroups:
      - certificates.k8s.io
    resources:
      - certificatesigningrequests/seedclient
    verbs:
      - create
---
# A kubelet/gardenlet authenticating using bootstrap tokens is authenticated as a user in the group system:bootstrappers
# Allows the Gardenlet to create a CSR
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gardener.cloud:system:seed-bootstrapper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:system:seed-bootstrapper
subjects:
  - kind: Group
    name: system:bootstrappers
    apiGroup: rbac.authorization.k8s.io
```

## Prepare the gardenlet Helm chart

This section only describes the minimal configuration, 
using the global configuration values of the gardenlet Helm chart. 
For an overview over all values, see the [configuration values](../../charts/gardener/gardenlet/values.yaml). 
We refer to the global configuration values as _gardenlet configuration_ in the remaining procedure.

1.  Create a gardenlet configuration `gardenlet-values.yaml` based on [this template](https://github.com/gardener/gardener/blob/master/charts/gardener/gardenlet/values.yaml).
    
2.  Create a bootstrap `kubeconfig` based on the bootstrap token created in the garden cluster.

    Replace the `<bootstrap-token>` with `token-id.token-secret` (from our previous example: `07401b.f395accd246ae52d`) from the bootstrap token secret.
    
    ```yaml
    apiVersion: v1
    kind: Config
    current-context: gardenlet-bootstrap@default
    clusters:
    - cluster:
        certificate-authority-data: <ca-of-garden-cluster>
        server: https://<endpoint-of-garden-cluster>
      name: default
    contexts:
    - context:
        cluster: default
        user: gardenlet-bootstrap
      name: gardenlet-bootstrap@default
    users:
    - name: gardenlet-bootstrap
      user:
        token: <bootstrap-token>
    ```

3.  In section `gardenClientConnection.bootstrapKubeconfig` of your gardenlet configuration, provide the bootstrap `kubeconfig` together with a name and namespace to the gardenlet Helm chart.
    
    ```yaml
    gardenClientConnection:
      bootstrapKubeconfig:
        name: gardenlet-kubeconfig-bootstrap
        namespace: garden
        kubeconfig: |
          <bootstrap-kubeconfig>  # will be base64 encoded by helm
    ```
    
    The bootstrap `kubeconfig` is stored in the specified secret.
    
4.  In section `gardenClientConnection.kubeconfigSecret` of your gardenlet configuration, 
    define a name and a namespace where the gardenlet stores 
    the real `kubeconfig` that it creates during the bootstrap process. If the secret doesn't exist, 
    the gardenlet creates it for you. 
    
    ```yaml
    gardenClientConnection:
      kubeconfigSecret:
        name: gardenlet-kubeconfig
        namespace: garden
    ```

## Automatically register shoot cluster as a seed cluster

A seed cluster can either be registered by manually creating 
the [`Seed` resource](../../example/50-seed.yaml) 
or automatically by the gardenlet.  
This functionality is useful for managed seed clusters, 
as the gardenlet in the garden cluster deploys a copy of itself 
into the cluster with automatic registration of the `Seed` configured.  
However, it can also be used to have a streamlined seed cluster registration process when manually deploying the gardenlet.

> This procedure doesn’t describe all the possible configurations 
> for the `Seed` resource. For more information, see:
> * [Example Seed resource](../../example/50-seed.yaml)
> * [Configurable Seed settings](../usage/seed_settings.md).

### Adjust the gardenlet component configuration

1. Supply the `Seed` resource in section `seedConfig` of your gardenlet configuration `gardenlet-values.yaml`.

    > Note that with the `seedConfig` supplied, the gardenlet is only responsible to create and reconcile this one configured seed (in the previous example: `sweet-seed`). The gardenlet can also be configured to reconcile (but not create!) multiple Seeds [based on a label selector](../concepts/gardenlet.md#seed-config-vs-seed-selector), which is only recommended [for a development setup](../development/local_setup.md#appendix).

1. Add the `seedConfig` to your gardenlet configuration `gardenlet-values.yaml`.
The field `seedConfig.spec.provider.type` specifies the infrastructure provider type (for example, `aws`) of the seed cluster.
For all supported infrastructure providers, see [Known Extension Implementations](../../extensions/README.md#known-extension-implementations).   

    ```yaml
    ....
    seedConfig:
      metadata:
        name: sweet-seed
      spec:
        dns:
          ingressDomain: ingress.sweet-seed.<my-domain> # see prerequisites
        networks: # see prerequisites
          nodes: 10.240.0.0/16
          pods: 100.244.0.0/16
          services: 100.32.0.0/13
          shootDefaults: # optional: non-overlapping default CIDRs for shoot clusters of that Seed
            pods: 100.96.0.0/11
            services: 100.64.0.0/13
        provider:
          region: eu-west-1
          type: <provider>
    ```

### Optional: Enable backup and restore

The seed cluster can be set up with backup and restore 
for the main `etcds` of shoot clusters. 

Gardener uses [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore)
that [integrates with different storage providers](https://github.com/gardener/etcd-backup-restore/blob/master/doc/usage/getting_started.md#usage) 
to store the shoot cluster's main `etcd` backups.
Make sure to obtain client credentials that have sufficient permissions with the chosen storage provider.

Create a secret in the garden cluster with client credentials for the storage provider.
The format of the secret is cloud provider specific and can be found 
in the repository of the respective Gardener extension. 
For example, the secret for AWS S3 can be found in the AWS provider extension 
([30-etcd-backup-secret.yaml](https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-etcd-backup-secret.yaml)).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sweet-seed-backup
  namespace: garden
type: Opaque
data:
  # client credentials format is provider specific
```

Configure the `Seed` resource in section `seedConfig` of your gardenlet configuration to use backup and restore:

```yaml
...
seedConfig:
  metadata:
    name: sweet-seed
  spec:
    backup:
      provider: <provider>
      secretRef:
        name: sweet-seed-backup
        namespace: garden
```

## Deploy the gardenlet

> The gardenlet doesn’t have to run in the same Kubernetes cluster as the seed cluster 
> it’s registering and reconciling, but it is in most cases advantageous 
> to use in-cluster communication to talk to the Seed API server. 
> Running a gardenlet outside of the cluster is mostly used for local development.

The `gardenlet-values.yaml` looks something like this 
(with automatic Seed registration and backup for shoot clusters enabled):

```yaml
global:
  # Gardenlet configuration values
  gardenlet:
    enabled: true
    ...
    <default config>
    ...
    config:
      gardenClientConnection:
        ...
        bootstrapKubeconfig:
          name: gardenlet-bootstrap-kubeconfig
          namespace: garden
          kubeconfig: |
            apiVersion: v1
            clusters:
            - cluster:
                certificate-authority-data: <dummy>
                server: <my-garden-cluster-endpoint>
              name: my-kubernetes-cluster
            ....
            
        kubeconfigSecret:
          name: gardenlet-kubeconfig
          namespace: garden
      ...
      <default config>
      ...
      seedConfig:
        metadata:
          name: sweet-seed
        spec:
          dns:
            ingressDomain: ingress.sweet-seed.<my-domain>
          networks:
            nodes: 10.240.0.0/16
            pods: 100.244.0.0/16
            services: 100.32.0.0/13
            shootDefaults:
              pods: 100.96.0.0/11
              services: 100.64.0.0/13
          provider:
            region: eu-west-1
            type: <provider>
          backup:
            provider: <provider>
            secretRef:
              name: sweet-seed-backup
              namespace: garden
```

Deploy the gardenlet Helm chart to the Kubernetes cluster.

```bash
helm install gardenlet charts/gardener/gardenlet \
  --namespace garden \
  -f gardenlet-values.yaml \
  --wait
```

This helm chart creates:

- A service account `gardenlet` that the gardenlet can use to talk to the Seed API server.
- RBAC roles for the service account (full admin rights at the moment).
- The secret (`garden`/`gardenlet-bootstrap-kubeconfig`) containing the bootstrap `kubeconfig`.
- The gardenlet deployment in the `garden` namespace.

## Check that the gardenlet is successfully deployed

1.  Check that the gardenlets certificate bootstrap was successful.
    
    Check if the secret `gardenlet-kubeconfig` in the namespace `garden` in the seed cluster
    is created and contains a `kubeconfig` with a valid certificate.
    
    1. Get the `kubeconfig` from the created secret.
    
        ```
        $ kubectl -n garden get secret gardenlet-kubeconfig -o json | jq -r .data.kubeconfig | base64 -d
        ```
    
    1. Test against the garden cluster and verify it’s working.
    
    1. Extract the `client-certificate-data` from the user `gardenlet`.
    
    1. View the certificate:
    
        ```
        $ openssl x509 -in ./gardenlet-cert -noout -text
        ```
    
        Check that the certificate is valid for a year (that is the lifetime of new certificates). 
    
2.  Check that the bootstrap secret `gardenlet-bootstrap-kubeconfig` has been deleted from the seed cluster in namespace `garden`.
    
3.  Check that the seed cluster is registered and `READY` in the garden cluster.
    
    Check that the seed cluster `sweet-seed` exists and all conditions indicate that it’s available.
    If so, the [Gardenlet is sending regular heartbeats](../concepts/gardenlet.md#heartbeats) and the [seed bootstrapping](../usage/seed_bootstrapping.md) was successful.
    
    Check that the conditions on the `Seed` resource look similar to the following:
    
    ```bash
    $ kubectl get seed sweet-seed -o json | jq .status.conditions
    [
      {
        "lastTransitionTime": "2020-07-17T09:17:29Z",
        "lastUpdateTime": "2020-07-17T09:17:29Z",
        "message": "Gardenlet is posting ready status.",
        "reason": "GardenletReady",
        "status": "True",
        "type": "GardenletReady"
      },
      {
        "lastTransitionTime": "2020-07-17T09:17:49Z",
        "lastUpdateTime": "2020-07-17T09:53:17Z",
        "message": "Seed cluster has been bootstrapped successfully.",
        "reason": "BootstrappingSucceeded",
        "status": "True",
        "type": "Bootstrapped"
      },
      {
        "lastTransitionTime": "2020-07-17T09:17:49Z",
        "lastUpdateTime": "2020-07-17T09:53:17Z",
        "message": "Backup Buckets are available.",
        "reason": "BackupBucketsAvailable",
        "status": "True",
        "type": "BackupBucketsReady"
      }
    ]
    ```

## Related Links

[Issue #1724: Harden Gardenlet RBAC privileges](https://github.com/gardener/gardener/issues/1724).

[Backup and Restore](../concepts/backup-restore.md).
