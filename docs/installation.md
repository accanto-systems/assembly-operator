# Installation

## From Release

Navigate to the releases page and download a deployment archive. 

Extract the archive (replace version to match your chosen release):

```
tar -xvzf assembly-operator-deployment-0.1.0.tgz
```

Quick install (read the [INSTALL.md](./dist-docs/INSTALL.md) included in the archive for more options):

```
cd assembly-operator-0.1.0
./apply.sh
```

## From Source

Run the following commands to install the Assembly Operator in to an existing Kubernetes cluster:

```
kubectl apply -f deploy/service_account.yaml
kubectl apply -f deploy/role.yaml
kubectl apply -f deploy/role_binding.yaml
kubectl apply -f deploy/crds/com_v1alpha1_assembly_crd.yaml
kubectl apply -f deploy/operator.yaml
```