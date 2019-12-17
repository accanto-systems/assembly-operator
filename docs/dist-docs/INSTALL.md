# Install

This requires `kubectl` access to a Kubernetes cluster.

## Quick Install

Installs the operator with the following settings:

- installed to the `default` namespace
- pulls docker image from dockerhub (accanto/assembly-operator)
- configured with connection to LM on internal host with default jack user (https://nimrod:8290)

```
./apply.sh
```

Set alternative namespace with first argument:

```
./apply.sh my-namespace
```

## Change LM Connection

Open `operator.yaml` and update the ConfigMap data:

```
data:
  config.yaml: |
    lmBase: https://nimrod:8290
    lmUsername: jack
    lmPassword: jack
```

Run `apply.sh`.

## Change docker image

Open `operator.yaml` and update the `image` under the `assembly-operator` container:

```
containers:
    - name: assembly-operator
      image: accanto/assembly-operator:0.1.0 ##change this
      command:
      - assembly-operator
      imagePullPolicy: Always
```

# Uninstall

**NOTE:** it is recommended that you remove all Assembly resources managed by the operator before uninstalling. 

Removes from the default namespace:

```
./remove.sh
```

Set alternative namespace with first argument:

```
./remove.sh my-namespace
```


# Usage

Read the [usage doc](./USAGE.md) to start creating Assemblies through the operator.