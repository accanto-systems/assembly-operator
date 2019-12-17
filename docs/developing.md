# Developing Assembly Operator

Assembly Operator is a [Kubernetes Operator](https://coreos.com/operators/) that is built using the [Kubernetes Operator SDK](https://github.com/operator-framework/operator-sdk).

Operator SDK uses [Go modules](https://github.com/golang/go/wiki/Modules), so make sure GO111MODULE is set to "on":

```
export GO111MODULE=on
```

## Building Assembly Operator

If you have changed anything in pkg/apis/com/v1alpha1/assembly_types.go, re-run the generare k8s command:

```
operator-sdk generate k8s
```

To build the operator binary and a Docker image:

```
operator-sdk build assembly-operator:0.1.0
```

Push the Docker image to a private Docker registry:

```
docker image tag assembly-operator:0.1.0 10.220.217.248:32736/assembly-operator:0.1.0
docker image push 10.220.217.248:32736/assembly-operator:0.1.0
```

or to Docker Hub:

```
docker image tag assembly-operator:0.1.0 accanto/assembly-operator:0.1.0
docker image push accanto/assembly-operator:0.1.0
```