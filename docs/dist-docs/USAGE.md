# Usage

Create Assembly by adding the following to a resource manifest file, i.e. myassembly.yaml:

```
apiVersion: com.accantosystems.stratoss/v1alpha1
kind: Assembly
metadata:
  name: MyAssembly
spec:
  assemblyName: "MyAssembly"
  descriptorName: "assembly::MyAssembly::1.0"
  intendedState: "Active"
  properties:
    properyA: "ValueA"
    resourceManager: "brent"
    deploymentLocation: "MyLocation"
```

NOTE: all values are examples. You must set the descriptorName and properties to valid values for your LM environment.

Apply with kubectl:

```
kubectl apply -f myassembly.yaml
```

Check status:

```
kubectl get assemblys
```

Or:

```
kubectl get assembly MyAssembly -o yaml
```

Make changes with `kubectl edit` or by modifying the resource manifest and re-applying it with `kubectl apply`.

```
kubectl edit assembly MyAssembly
```

Remove an Assembly:

```
kubectl delete assembly MyAssembly
```