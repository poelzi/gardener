# Scoped API Access for Gardenlets

By default, `gardenlet`s have administrative access in the garden cluster.
They are able to execute any API request on any object independent of whether the object is related to the seed cluster the `gardenlet` is responsible fto.
As RBAC is not powerful enough for fine-grained checks and for the sake of security, Gardener provides two optional but recommended configurations for your environments that scope the API access for `gardenlet`s.

Similar to the [`Node` authorization mode in Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/node/), Gardener features a `SeedAuthorizer` plugin.
It is a special-purpose authorization plugin that specifically authorizes API requests made by the `gardenlet`s.

Likewise, similar to the [`NodeRestriction` admission plugin in Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction), Gardener features a `SeedRestriction` plugin.
It is a special-purpose admission plugin that specifically limits the Kubernetes objects `gardenlet`s can modify.

📚 You might be interested to look into the [design proposal for scoped Kubelet API access](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/kubelet-authorizer.md) from the Kubernetes community.
It can be translated to Gardener and Gardenlets with their `Seed` and `Shoot` resources.

## Flow Diagram

The following diagram shows how the two plugins are included in the request flow of a `gardenlet`.
When they are not enabled then the `kube-apiserver` is internally authorizing the request via RBAC before forwarding the request directly to the `gardener-apiserver`, i.e., the `gardener-admission-controller` would not be consulted (this is not entirely correct because it also serves other admission webhook handlers, but for simplicity reasons this document focuses on the API access scope only).

When enabling the plugins, there is one additional step for each before the `gardener-apiserver` responds to the request.

![Flow Diagram](gardenlet_api_access_flow.png)

Please note that the example shows a request to an object (`Shoot`) residing in one of the API groups served by `gardener-apiserver`.
However, the `gardenlet` is also interacting with objects in API groups served by the `kube-apiserver` (e.g., `Secret`,`ConfigMap`, etc.).
In this case, the consultation of the `SeedRestriction` admission plugin is performed by the `kube-apiserver` itself before it forwards the request to the `gardener-apiserver`.

Today, the following rules are implemented:

| Resource                 | Verbs                                                         | Path                                          | Description |
| ------------------------ | ------------------------------------------------------------- | ----------------------------------------------| ----------- |
| `BackupBucket`           | `get`, `list`, `watch`, `create`, `update`, `patch`, `delete` | `BackupBucket` -> `Seed`                      | Allow `get`, `list`, `watch` requests for all `BackupBucket`s. Allow only `create`, `update`, `patch`, `delete` requests for `BackupBucket`s assigned to the `gardenlet`'s `Seed`. |
| `BackupEntry`            | `get`, `list`, `watch`, `create`, `update`, `patch`           | `BackupEntry` -> `Seed`                       | Allow `get`, `list`, `watch` requests for all `BackupEntry`s. Allow only `create`, `update`, `patch` requests for `BackupEntry`s assigned to the `gardenlet`'s `Seed` and referencing `BackupBucket`s assigned to the `gardenlet`'s `Seed`. |
| `CloudProfile`           | `get`                                                         | `CloudProfile` -> `Shoot` -> `Seed`           | Allow only `get` requests for `CloudProfile`s referenced by `Shoot`s that are assigned to the `gardenlet`'s `Seed`. |
| `ConfigMap`              | `get`                                                         | `ConfigMap` -> `Shoot` -> `Seed`              | Allow only `get` requests for `ConfigMap`s referenced by `Shoot`s that are assigned to the `gardenlet`'s `Seed`. Allows reading the `kube-system/cluster-identity` `ConfigMap`. |
| `ControllerRegistration` | `get`, `list`, `watch`                                        | none                                          | Allow `get`, `list`, `watch` requests for all `ControllerRegistration`s. |
| `ControllerInstallation` | `get`, `list`, `watch`, `update`, `patch`                     | `ControllerInstallation` -> `Seed`            | Allow `get`, `list`, `watch` requests for all `ControllerInstallation`s. Allow only `update`, `patch` requests for `ControllerInstallation`s assigned to the `gardenlet`'s `Seed`. |
| `Event`                  | `create`                                                      | none                                          | Allow to create all kinds of `Event`s. |
| `Lease`                  | `create`, `get`, `watch`, `update`                            | `Lease` -> `Seed`                             | Allow `create`, `get`, `update`, and `delete` requests for `Lease`s of the `gardenlet`'s `Seed`. |
| `ManagedSeed`            | `get`, `list`, `watch`, `update`, `patch`                     | `ManagedSeed` -> `Shoot` -> `Seed`            | Allow `get`, `list`, `watch` requests for all `ManagedSeed`s. Allow only `update`, `patch` requests for `ManagedSeed`s referencing a `Shoot` assigned to the `gardenlet`'s `Seed`. |
| `Namespace`              | `get`                                                         | `Namespace` -> `Shoot` -> `Seed`              | Allow `get` requests for `Namespace`s of `Shoot`s that are assigned to the `gardenlet`'s `Seed`. |
| `Project`                | `get`                                                         | `Project` -> `Namespace` -> `Shoot` -> `Seed` | Allow `get` requests for `Project`s referenced by the `Namespace` of `Shoot`s that are assigned to the `gardenlet`'s `Seed`. |
| `SecretBinding`          | `get`                                                         | `SecretBinding` -> `Shoot` -> `Seed`          | Allow only `get` requests for `SecretBinding`s referenced by `Shoot`s that are assigned to the `gardenlet`'s `Seed`. |
| `Seed`                   | `get`, `list`, `watch`, `create`, `update`, `patch`, `delete` | `Seed`                                        | Allow `get`, `list`, `watch` requests for all `Seed`s. Allow only `create`, `update`, `patch`, `delete` requests for the `gardenlet`'s `Seed`s. [1] |
| `Shoot`                  | `get`, `list`, `watch`, `update`, `patch`                     | `Shoot` -> `Seed`                             | Allow `get`, `list`, `watch` requests for all `Shoot`s. Allow only `update`, `patch` requests for `Shoot`s assigned to the `gardenlet`'s `Seed`. |
| `ShootState`             | `get`, `create`, `update`, `patch`                            | `ShootState` -> `Shoot` -> `Seed`             | Allow only `get`, `create`, `update`, `patch` requests for `ShootState`s belonging by `Shoot`s that are assigned to the `gardenlet`'s `Seed`. |

[1] If you use `ManagedSeed` resources then the gardenlet reconciling them ("parent gardenlet") may be allowed to submit certain requests for the `Seed` resources resulting out of such `ManagedSeed` reconciliations (even if the "parent gardenlet" is not responsible for them): 

- ℹ️ It is allowed to delete the `Seed` resources if the corresponding `ManagedSeed` objects already have a `deletionTimestamp` (this is secure as gardenlets themselves don't have permissions for deleting `ManagedSeed`s).
- ⚠ It is allowed to create or update `Seed` resources if the corresponding `ManagedSeed` objects use a seed template, i.e., `.spec.seedTemplate != nil`. In this case, there is at least one gardenlet in your system which is responsible for two or more `Seed`s. Please keep in mind that this use case is not recommended for production scenarios (you should only have one dedicated gardenlet per seed cluster), hence, the security improvements discussed in this document might be limited.   

## `SeedAuthorizer` Authorization Webhook Enablement

The `SeedAuthorizer` is implemented as [Kubernetes authorization webhook](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) and part of the [`gardener-admission-controller`](../concepts/admission-controller.md) component running in the garden cluster.

⚠️ This authorization plugin is still in development and should not be used yet.

### Authorizer Decisions

As mentioned earlier, it's the authorizer's job to evaluate API requests and return one of the following decisions:

- `DecisionAllow`: The request is allowed, further configured authorizers won't be consulted.
- `DecisionDeny`: The request is denied, further configured authorizers won't be consulted.
- `DecisionNoOpinion`: A decision cannot be made, further configured authorizers will be consulted.

For backwards compatibility, no requests are denied at the moment, so that an ambiguous request is still deferred to a subsequent authorizer like RBAC.

First, the `SeedAuthorizer` extracts the `Seed` name from the API request. This requires a proper TLS certificate the `gardenlet` uses to contact the API server and is automatically given if [TLS bootstrapping](../concepts/gardenlet.md#TLS-Bootstrapping) is used.
Concretely, the authorizer checks the certificate for name `gardener.cloud:system:seed:<seed-name>` and group `gardener.cloud:system:seeds`.
In cases this information is missing e.g., when a custom Kubeconfig is used, the authorizer cannot make any decision.
Likewise, if `gardenlet` is responsible for more than one `Seed`, the name in the mentioned TLS certificate is `gardener.cloud:system:seed:<ambiguous>` and a definite decision cannot be made as well.
The authorizer immediately returns with `DecisionNoOpinion` for all ambiguous cases which means that the request is neither allowed nor denied and further configured authorizers (e.g. RBAC) will be contacted.
Thus, RBAC is still a considerable option to restrict the `gardenlet`'s access permission if the above explained preconditions are not given.

With the `Seed` name at hand, the authorizer checks for an **existing path** from the resource that a request is being made for to the `Seed` belonging to the `gardenlet`. Take a look at the [Implementation Details](#implementation-details) section for more information.

### Implementation Details

Internally, the `SeedAuthorizer` uses a directed, acyclic graph data structure in order to efficiently respond to authorization requests for gardenlets:

* A vertex in this graph represents a Kubernetes resource with its kind, namespace, and name (e.g., `Shoot:garden-my-project/my-shoot`).
* An edge from vertex `u` to vertex `v` in this graph exists when (1) `v` is referred by `u` and `v` is a `Seed`, or when (2) `u` is referred by `v`.

For example, a `Shoot` refers to a `Seed`, a `CloudProfile`, a `SecretBinding`, etc., so it has an outgoing edge to the `Seed` (1) and incoming edges from the `CloudProfile` and `SecretBinding` vertices (2).

![Resource Dependency Graph](gardenlet_api_access_graph.png)

In above picture the resources that are actively watched have are shaded.
Gardener resources are green while Kubernetes resources are blue.
It shows the dependencies between the resources and how the graph is built based on above rules.

ℹ️ Above picture shows all resources that may be accessed by `gardenlet`s, except for the `Quota` resource which is only included for completeness.

Now, when a `gardenlet` wants to access certain resources then the `SeedAuthorizer` uses a Depth-First traversal starting from the vertex representing the resource in question, e.g., from a `Project` vertex.
If there is a path from the `Project` vertex to the vertex representing the `Seed` the gardenlet is responsible for then it allows the request.

#### Metrics

The `SeedAuthorizer` registers the following metrics related to the mentioned graph implementation:

| Metric | Description |
| --- | --- |
| `gardener_admission_controller_seed_authorizer_graph_update_duration_seconds` | Histogram of duration of resource dependency graph updates in seed authorizer, i.e., how long does it take to update the graph's vertices/edges when a resource is created, changed, or deleted. |
| `gardener_admission_controller_seed_authorizer_graph_path_check_duration_seconds` | Histogram of duration of checks whether a path exists in the resource dependency graph in seed authorizer. |

#### Debug Handler

When the `.server.enableDebugHandlers` field in the `gardener-admission-controller`'s component configuration is set to `true` then it serves a handler that can be used for debugging the resource dependency graph under `/debug/resource-dependency-graph`.

🚨 Only use this setting for development purposes as it enables unauthenticated users to view all data if they have access to the `gardener-admission-controller` component.

The handler renders an HTML page displaying the current graph with a list of vertices and its associated incoming and outgoing edges to other vertices.
Depending on the size of the Gardener landscape (and consequently, the size of the graph), it might not be possible to render it in its entirety.
If there are more than 2000 vertices then the default filtering will selected for `kind=Seed` to prevent overloading the output.

_Example output_:

```text
-------------------------------------------------------------------------------
|
| # Seed:my-seed
|   <- (11)
|     BackupBucket:73972fe2-3d7e-4f61-a406-b8f9e670e6b7
|     BackupEntry:garden-my-project/shoot--dev--my-shoot--4656a460-1a69-4f00-9372-7452cbd38ee3
|     ControllerInstallation:dns-external-mxt8m
|     ControllerInstallation:extension-shoot-cert-service-4qw5j
|     ControllerInstallation:networking-calico-bgrb2
|     ControllerInstallation:os-gardenlinux-qvb5z
|     ControllerInstallation:provider-gcp-w4mvf
|     Secret:garden/backup
|     Shoot:garden-my-project/my-shoot
|
-------------------------------------------------------------------------------
|
| # Shoot:garden-my-project/my-shoot
|   <- (5)
|     CloudProfile:gcp
|     Namespace:garden-my-project
|     Secret:garden-my-project/my-dns-secret
|     SecretBinding:garden-my-project/my-credentials
|     ShootState:garden-my-project/my-shoot
|   -> (1)
|     Seed:my-seed
|
-------------------------------------------------------------------------------
|
| # ShootState:garden-my-project/my-shoot
|   -> (1)
|     Shoot:garden-my-project/my-shoot
|
-------------------------------------------------------------------------------

... (etc., similarly for the other resources)
```

There are anchor links to easily jump from one resource to another, and the page provides means for filtering the results based on the `kind`, `namespace`, and/or `name`.

#### Pitfalls

When there is a relevant update to an existing resource, i.e., when a reference to another resource is changed, then the corresponding vertex (along with all associated edges) is first deleted from the graph before it gets added again with the up-to-date edges.
However, this does only work for vertices belonging to resources that are only created in exactly one "watch handler".
For example, the vertex for a `SecretBinding` can either be created in the `SecretBinding` handler itself or in the `Shoot` handler.
In such cases, deleting the vertex before (re-)computing the edges might lead to race conditions and potentially renders the graph invalid.
Consequently, instead of deleting the vertex, only the edges the respective handler is responsible for are deleted.
If the vertex ends up with no remaining edges then it also gets deleted automatically.
Afterwards, the vertex can either be added again or the updated edges can be created.

## `SeedRestriction` Admission Webhook Enablement

The `SeedRestriction` is implemented as [Kubernetes admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) and part of the [`gardener-admission-controller`](../concepts/admission-controller.md) component running in the garden cluster.

⚠️ This admission plugin is still in development and should not be used yet.
