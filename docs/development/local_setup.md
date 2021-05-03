# Preparing the Setup

Conceptually, all Gardener components are designated to run as a Pod inside a Kubernetes cluster.
The API server extends the Kubernetes API via the user-aggregated API server concepts.
However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time.
That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details could be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture)

This setup is based on [minikube](https://github.com/kubernetes/minikube), a Kubernetes cluster running on a single node. Docker for Desktop and [kind](https://github.com/kubernetes-sigs/kind) are also supported.

## Installing Golang environment

Install latest version of Golang. For MacOS you could use [Homebrew](https://brew.sh/):

```bash
brew install go
```

For other OS, please check [Go installation documentation](https://golang.org/doc/install).

## Installing kubectl and helm

As already mentioned in the introduction, the communication with the Gardener happens via the Kubernetes (Garden) cluster it is targeting. To interact with that cluster, you need to install `kubectl`. Please make sure that the version of `kubectl` is at least `v1.11.x`.

On MacOS run

```bash
brew install kubernetes-cli
```

Please check the [kubectl installation documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for other OS.

You may also need to develop Helm charts or interact with Tiller using the [Helm](https://github.com/kubernetes/helm) CLI:

On MacOS run

```bash
brew install kubernetes-helm
```

On other OS please check the [Helm installation documentation](https://helm.sh/docs/intro/install/).

## Installing git

We use `git` as VCS which you need to install.

On MacOS run

```bash
brew install git
```

On other OS, please check the [Git installation documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

## Installing openvpn

We use `OpenVPN` to establish network connectivity from the control plane running in the Seed cluster to the Shoot's worker nodes running in private networks.
To harden the security we need to generate another secret to encrypt the network traffic ([details](https://openvpn.net/index.php/open-source/documentation/howto.html#security)).
Please install the `openvpn` binary.

On MacOS run

```bash
brew install openvpn
export PATH=$(brew --prefix openvpn)/sbin:$PATH
```

On other OS, please check the [OpenVPN downloads page](https://openvpn.net/index.php/open-source/downloads.html).

## Installing Minikube

You'll need to have [minikube](https://github.com/kubernetes/minikube#installation) installed and running.

On MacOS run

```bash
brew install minikube
```

> Note: Gardener is working only with self-contained kubeconfig files because of [security issue](https://banzaicloud.com/blog/kubeconfig-security/). You can configure your minikube to create self-contained kubeconfig files via:
> ```bash
> minikube config set embed-certs true
> ```

Alternatively, you can also install Docker for Desktop and [kind](https://github.com/kubernetes-sigs/kind).

In case you want to use the "Docker for Mac Kubernetes" or if you want to build Docker images for the Gardener you have to install Docker itself. On MacOS, please use [Docker for MacOS](https://docs.docker.com/docker-for-mac/) which can be downloaded [here](https://download.docker.com/mac/stable/Docker.dmg).

On other OS, please check the [Docker installation documentation](https://docs.docker.com/install/).

## Installing iproute2

`iproute2` provides a collection of utilities for network administration and configuration.

On MacOS run

```bash
brew install iproute2mac
```

## Installing yaml2json and jq

```bash
go get -u github.com/bronze1man/yaml2json
brew install jq
export PATH=$PATH:$(go env GOPATH)/bin
```

## Installing GNU Parallel

[GNU Parallel](https://www.gnu.org/software/parallel/) is a shell tool for executing jobs in parallel, used by the code generation scripts (`make generate`).

On MacOS run

```bash
brew install parallel
```

## [MacOS only] Install GNU core utilities

When running on MacOS you have to install the GNU core utilities:

```bash
brew install coreutils gnu-sed
```

This will create symbolic links for the GNU utilities with `g` prefix in `/usr/local/bin`, e.g., `gsed` or `gbase64`. To allow using them without the `g` prefix please put `/usr/local/opt/coreutils/libexec/gnubin` at the beginning of your `PATH` environment variable, e.g., `export PATH=/usr/local/opt/coreutils/libexec/gnubin:$PATH`.

## [Windows] WSL2

Apart from Linux distributions and MacOS, the local gardener setup can also run on the Windows Subsystem for Linux 2. 

While WSL1, plain docker for windows and various Linux distributions and local Kubernetes environments may be supported, this setup was verified with:
* [WSL2](https://docs.microsoft.com/en-us/windows/wsl/wsl2-index) 
* [Docker Desktop WSL2 Engine](https://docs.docker.com/docker-for-windows/wsl/)
* [Ubuntu 18.04 LTS on WSL2](https://ubuntu.com/blog/ubuntu-on-wsl-2-is-generally-available)  
* Nodeless local garden (see below)

The Gardener repository and all the above-mentioned tools (git, golang, kubectl, ...) should be installed in your WSL2 distro, according to the distribution-specific Linux installation instructions. 

## [Optional] Installing gcloud SDK

In case you have to create a new release or a new hotfix of the Gardener you have to push the resulting Docker image into a Docker registry. Currently, we are using the Google Container Registry (this could change in the future). Please follow the official [installation instructions from Google](https://cloud.google.com/sdk/downloads).

## [Optional] Install GNU screen

When screen is installed, you can easily start all local daemons using `make start-all`
To install screen on *MacOS*

```bash
brew install screen
```

To install screen on *Debian/Ubuntu*

```bash
apt install screen
```

## NixOS/Nix support

Gardner contains a `shell.nix` file for development purposes. The nix package manager
can be installed parallel and provides some [unique features](https://nixos.org/guides/how-nix-works.html). All the required dependencies will be automatically installed when your run:

```bash
nix-shell
```

You can use this environment from a IDE with commands like:

```bash
nix-shell --command "make generate"
```

## Local Gardener setup

This setup is only meant to be used for developing purposes, which means that only the control plane of the Gardener cluster is running on your machine.

### Get the sources

Clone the repository from GitHub into your `$GOPATH`.

```bash
mkdir -p $GOPATH/src/github.com/gardener
cd $GOPATH/src/github.com/gardener
git clone git@github.com:gardener/gardener.git
cd gardener
```

> Note: Gardener is using Go modules and cloning the repository into `$GOPATH` is not a hard requirement. However it is still recommended to clone into `$GOPATH` because `k8s.io/code-generator` does not work yet outside of `$GOPATH` - [kubernetes/kubernetes#86753](https://github.com/kubernetes/kubernetes/issues/86753).

### Start the Gardener

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture), and what the various clusters are used for.

#### Start a local kubernetes cluster

For the development of Gardener you need some kind of Kubernetes cluster, which can be used as a "garden" cluster.
I.e. you need a Kubernetes API server on which you can register a `APIService` Gardener's own Extension API Server.  
For this you can use a standard tool from the community to setup a local cluster like minikube, kind or the Kubernetes Cluster feature in Docker for Desktop.

However, if you develop and run Gardener's components locally, you don't actually need a fully fledged Kubernetes Cluster,
i.e. you don't actually need to run Pods on it. If you want to use a more lightweight approach for development purposes,
you can use the "nodeless Garden cluster setup" residing in `hack/local-garden`. This is the easiest way to get your
Gardener development setup up and running.

**Using the nodeless cluster setup**

Setting up a local nodeless Garden cluster is quite simple. The only prerequisite is a running docker daemon.
Just use the provided Makefile rules to start your local Garden:
```bash
make local-garden-up
[...]
Starting gardener-dev kube-etcd cluster..!
Starting gardener-dev kube-apiserver..!
Starting gardener-dev kube-controller-manager..!
Starting gardener-dev gardener-etcd cluster..!
namespace/garden created
clusterrole.rbac.authorization.k8s.io/gardener.cloud:admin created
clusterrolebinding.rbac.authorization.k8s.io/front-proxy-client created
[...]
```

This will start all minimally required components of a Kubernetes cluster (`etcd`, `kube-apiserver`, `kube-controller-manager`)
and an `etcd` Instance for the `gardener-apiserver` as Docker containers.

ℹ️ [Optional] If you want to develop the `SeedAuthorization` feature then you have to run `make ACTIVATE_SEEDAUTHORIZER=true local-garden-up`. However, please note that this forces you to start the `gardener-admission-controller` via `make start-admission-controller`.

To tear down the local Garden cluster and remove the Docker containers, simply run:
```bash
make local-garden-down
```

**Using minikube**

Alternatively, spin up a cluster with minikube with this command:

```bash
minikube start --embed-certs #  `--embed-certs` can be omitted if minikube has already been set to create self-contained kubeconfig files.
😄  minikube v1.8.2 on Darwin 10.15.3
🔥  Creating virtualbox VM (CPUs=2, Memory=2048MB, Disk=20000MB) ...
[...]
🏄  Done! Thank you for using minikube!
```

**Using a remote cluster as Garden cluster**

For some testing scenarios, you may want to use a remote cluster instead of a local one as your Garden cluster. 
To do this, you can use the "remote Garden cluster setup" residing in `hack/remote-garden`. 
To avoid mistakes, the remote cluster must have a `garden` namespace labeled with `gardener.cloud/purpose=remote-garden`. 
You must create the `garden` namespace and label it manually before running `make remote-garden-up` as described below.

Use the provided `Makefile` rules to bootstrap your remote Garden:

```bash
export KUBECONFIG=<path to kubeconfig>
make remote-garden-up
[...]
# Start gardener etcd used to store gardener resources (e.g., seeds, shoots)
Starting gardener-dev-remote gardener-etcd cluster!
[...]
# Open tunnels for accessing local gardener components from the remote cluster
[...]
```

This will start an `etcd` instance for the `gardener-apiserver` as a Docker container, and open tunnels for accessing local gardener components from the remote cluster.

To close the tunnels and remove the locally-running Docker containers, run:

```bash
make remote-garden-down
```

> Note: The minimum K8S version of the remote cluster that can be used as Garden cluster is `1.19.x`.

> ⚠️ Please be aware that in the remote garden setup all Gardener components run with administrative permissions, i.e., there is no fine-grained access control via RBAC (as opposed to productive installations of Gardener).

#### Prepare the Gardener

Now, that you have started your local cluster, we can go ahead and register the Gardener API Server.
Just point your `KUBECONFIG` environment variable to the local cluster you created in the previous step and run:

```bash
make dev-setup
Found Minikube ...
namespace/garden created
namespace/garden-dev created
deployment.apps/etcd created
service/etcd created
service/gardener-apiserver created
service/gardener-admission-controller created
endpoints/gardener-apiserver created
endpoints/gardener-admission-controller created
apiservice.apiregistration.k8s.io/v1alpha1.core.gardener.cloud created
apiservice.apiregistration.k8s.io/v1beta1.core.gardener.cloud created
apiservice.apiregistration.k8s.io/v1alpha1.seedmanagement.gardener.cloud created
apiservice.apiregistration.k8s.io/v1alpha1.settings.gardener.cloud created
```

Optionally, you can switch off the `Logging` feature gate of Gardenlet to save resources:

```bash
sed -i -e 's/Logging: true/Logging: false/g' dev/20-componentconfig-gardenlet.yaml
```

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`.
In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/10-secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../usage/configuration.md).

```bash
kubectl apply -f example/10-secret-internal-domain-unmanaged.yaml
secret/internal-domain-unmanaged created
```

#### Run the Gardener

Next, run the Gardener API Server, the Gardener Controller Manager (optionally), the Gardener Scheduler (optionally), and the Gardenlet in different terminal windows/panes using rules in the `Makefile`.

```bash
make start-apiserver
Found Minikube ...
I0306 15:23:51.044421   74536 plugins.go:84] Registered admission plugin "ResourceReferenceManager"
I0306 15:23:51.044523   74536 plugins.go:84] Registered admission plugin "DeletionConfirmation"
[...]
I0306 15:23:51.626836   74536 secure_serving.go:116] Serving securely on [::]:8443
[...]
```

(Optional) Now you are ready to launch the Gardener Controller Manager.

```bash
make start-controller-manager
time="2019-03-06T15:24:17+02:00" level=info msg="Starting Gardener controller manager..."
time="2019-03-06T15:24:17+02:00" level=info msg="Feature Gates: "
time="2019-03-06T15:24:17+02:00" level=info msg="Starting HTTP server on 0.0.0.0:2718"
time="2019-03-06T15:24:17+02:00" level=info msg="Acquired leadership, starting controllers."
time="2019-03-06T15:24:18+02:00" level=info msg="Starting HTTPS server on 0.0.0.0:2719"
time="2019-03-06T15:24:18+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2019-03-06T15:24:18+02:00" level=info msg="Successfully bootstrapped the Garden cluster."
time="2019-03-06T15:24:18+02:00" level=info msg="Gardener controller manager (version 1.0.0-dev) initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="ControllerRegistration controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="SecretBinding controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Project controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Quota controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="CloudProfile controller initialized."
[...]
```

(Optional) Now you are ready to launch the Gardener Scheduler.

```bash
make start-scheduler
time="2019-05-02T16:31:50+02:00" level=info msg="Starting Gardener scheduler ..."
time="2019-05-02T16:31:50+02:00" level=info msg="Starting HTTP server on 0.0.0.0:10251"
time="2019-05-02T16:31:50+02:00" level=info msg="Acquired leadership, starting scheduler."
time="2019-05-02T16:31:50+02:00" level=info msg="Gardener scheduler initialized (with Strategy: SameRegion)"
time="2019-05-02T16:31:50+02:00" level=info msg="Scheduler controller initialized."
[...]
```

(Optional) Now you are ready to launch the Gardenlet.

```bash
make start-gardenlet
time="2019-11-06T15:24:17+02:00" level=info msg="Starting Gardenlet..."
time="2019-11-06T15:24:17+02:00" level=info msg="Feature Gates: HVPA=true, Logging=true"
time="2019-11-06T15:24:17+02:00" level=info msg="Acquired leadership, starting controllers."
time="2019-11-06T15:24:18+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2019-11-06T15:24:18+02:00" level=info msg="Gardenlet (version 1.0.0-dev) initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="ControllerInstallation controller initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="Shoot controller initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="Seed controller initialized."
[...]
```

:warning: The Gardenlet will handle all your seeds for this development scenario, although, for productive usage it is recommended to run it once per seed, see [this document](../concepts/gardenlet.md) for more information.
See the [Appendix](#appendix) on how to configure the Seed clusters for the local development scenario. 

Please checkout the [Gardener Extensions Manager](https://github.com/gardener/gem) to install extension controllers - make sure that you install all of them required for your local development.
Also, please refer to [this document](../extensions/controllerregistration.md) for further information about how extensions are registered in case you want to use other versions than the latest releases.

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
kubectl get shoots
No resources found.
```

to operate against your local running Gardener API Server.

> Note: It may take several seconds until the `minikube` cluster recognizes that the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

### Create a Shoot

The steps below describe the general process of creating a Shoot. Have in mind that the steps do not provide full example manifests. The reader needs to check the provider documentation and adapt the manifests accordingly.

#### 1. Copy the example manifests to dev/

The next steps require modifications of the example manifests. These modifications are part of local setup and should not be `git push`-ed. To do not interfere with git, let's copy the example manifests to `dev/` which is ignored by git.

```bash
cp example/*.yaml dev/
```

#### 2. Create a Project

Every Shoot is associated with a Project. Check the corresponding example manifests `dev/00-namespace-garden-dev.yaml` and `dev/05-project-dev.yaml`. Adapt them and create them.

```bash
kubectl apply -f dev/00-namespace-garden-dev.yaml
kubectl apply -f dev/05-project-dev.yaml
```

Make sure that the Project is successfully reconciled:

```bash
$ kubectl get project dev
NAME   NAMESPACE    STATUS   OWNER                  CREATOR            AGE
dev    garden-dev   Ready    john.doe@example.com   kubernetes-admin   6s
```

#### 3. Create a CloudProfile

The `CloudProfile` resource is provider specific and describes the underlying cloud provider (available machine types, regions, machine images, etc.). Check the corresponding example manifest `dev/30-cloudprofile.yaml`. Check also the documentation and example manifests of the provider extension. Adapt `dev/30-cloudprofile.yaml` and apply it. 

```bash
kubectl apply -f dev/30-cloudprofile.yaml
```

#### 4. Create the required ControllerRegistrations

The [Known Extension Implementations](../../extensions/README.md#known-extension-implementations) section contains a list of available extension implementations. You need to create a ControllerRegistration for at least one infrastructure provider, dns provider (if the DNS for the Seed is not disabled), at least one operating system extension and at least one network plugin extension.
As a convention, example ControllerRegistration manifest for an extension is located under `example/controller-registration.yaml` in the corresponding repository (for example for AWS the ControllerRegistration can be found [here](https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/controller-registration.yaml)). An example creation of ControllerRegistration for provider-aws:

```bash
kubectl apply -f https://raw.githubusercontent.com/gardener/gardener-extension-provider-aws/master/example/controller-registration.yaml
```

#### 5. Register a Seed

When using the Gardenlet in a local development scenario with `make start-gardenlet` then the Gardenlet component configuration is setup with a [seed selector](../concepts/gardenlet.md#seed-config-vs-seed-selector) that targets all available Seed clusters.
However, a `Seed` resource needs to be configured to allow being reconciled by a Gardenlet which such a configuration.

When deploying the Gardenlet to reconcile only one Seed cluster (using component configuration `.seedConfig`), 
the Gardenlet either needs to be supplied with a kubeconfig for the particular Seed cluster, or acquires one via bootstrapping.
Having said that, if the Gardenlet is configured to manage multiple Seed clusters based on a label selector, it needs to fetch the kubeconfig of each Seed cluster at runtime from somewhere.
That is why the `Seed` resource needs to be configured with an additional secret reference that contains the kubeconfig of the Seed cluster.

Check the corresponding example manifest `dev/40-secret-seed.yaml` and `dev/50-seed.yaml`. Update `dev/40-secret-seed.yaml` with base64 encoded kubeconfig of the cluster that will be used as Seed (the scope of the permissions should be identical to the kubeconfig that the Gardenlet creates during bootstrapping - for now, `cluster-admin` privileges are recommended).

```bash
kubectl apply -f dev/40-secret-seed.yaml
```

Adapt `dev/50-seed.yaml` - adjust `.spec.secretRef` to refer the newly created Secret, adjust `.spec.provider` with the Seed cluster provider and revise the other fields.

```bash
kubectl apply -f dev/50-seed.yaml
```

Make sure that the Seed is successfully reconciled:

```bash
kubectl get seed
NAME       STATUS    PROVIDER    REGION      AGE    VERSION       K8S VERSION
seed-aws   Ready     aws         eu-west-1   4m     v1.11.0-dev   v1.17.12
```

### 6. Create a Shoot

A Shoot requires a SecretBinding. The SecretBinding refers to a Secret that contains the cloud provider credentials. The Secret data keys are provider specific and you need to check the documentation of the provider to find out which data keys are expected (for example for AWS the related documentation can be found [here](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md#provider-secret-data)). Adapt `dev/70-secret-provider.yaml` and `dev/80-secretbinding.yaml` and apply them.

```bash
kubectl apply -f dev/70-secret-provider.yaml
kubectl apply -f dev/80-secretbinding.yaml
```

After the SecretBinding creation, you are ready to proceed with the Shoot creation. You need to check the documentation of the provider to find out the expected configuration (for example for AWS the related documentation and example Shoot manifest can be found [here](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md)). Adapt `dev/90-shoot.yaml` and apply it.

To make sure that a specific Seed cluster will be chosen or to skip the scheduling (the sheduling requires Gardener Scheduler to be running), specify the `.spec.seedName` field (see [here](../../example/90-shoot.yaml#L317-L318)).

```bash
kubectl apply -f dev/90-shoot.yaml
```

Watch the progress of the operation and make sure that the Shoot will be successfully created.

```bash
watch kubectl get shoot --all-namespaces
```

#### Limitations of local development setup

You can run Gardener (API server, controller manager, scheduler, gardenlet) against any local Kubernetes cluster, however, your seed and shoot clusters must be deployed to a "real" provider.
Currently, it is not possible to run Gardener entirely isolated from any cloud provider.
We are planning to support a setup that can run completely locally (see [this for details](https://github.com/gardener/gardener-extension-provider-mock)), however, it does not yet exist.
This means that - after you have setup Gardener - you need to register an external seed cluster (e.g., one created in AWS).
Only after that step you can start creating shoot clusters with your locally running Gardener.
