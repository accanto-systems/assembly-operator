# Using Assembly Operator

Create an assembly instance (in this case an IP PBX) in [LM](http://servicelifecyclemanager.com/2.1.0/) by running:

```
kubectl apply -f ippbx-k8s.yaml
```

where ippbx-k8s.yaml looks like this:

```
apiVersion: com.accantosystems.stratoss/v1alpha1
kind: Assembly
metadata:
  name: ippbx1
spec:
  AssemblyName:   "ippbx1"
  DescriptorName: "assembly::ip-pbx::1.0"
  IntendedState:  "Active"
  Properties:
    resourceManager: "brent"
    deploymentLocation: "highgarden"
```

This assumes that LM has:

* an onboarded [K8s VIM driver](https://github.com/accanto-systems/k8s-vim-driver) using [lmctl](http://servicelifecyclemanager.com/2.1.0/user-guides/resource-engineering/develop-vim-driver/onboard-a-vim-driver/)

e.g.

```
lmctl vimdriver add --type Kubernetes --url http://k8s-vim-driver:8294 dev
```

* an onboarded deployment location called "highgarden" type "Kubernetes" linked to the onboarded VIM driver.

e.g.

```
lmctl deployment add -r brent -i Kubernetes -p highgarden.json dev highgarden
```

* an onboarded IP-PBX VNF["assembly::ip-pbx-sg::1.0"](https://github.com/accanto-systems/marketplace/tree/port-heat/vnfs/ip-pbx) using [lmctl](http://servicelifecyclemanager.com/2.1.0/user-guides/cicd/create-release-pipeline/)

e.g.

```
lmctl project push dev
```