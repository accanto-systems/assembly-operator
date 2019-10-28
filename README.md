[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Assembly Operator is a [K8s Operator](https://coreos.com/operators/) that provides a K8s API to manage the lifecycle of services running in an [LM](http://servicelifecyclemanager.com/) deployment.

# Restrictions

* currently, only create assembly is supported (no support for deletes or updates).
* currently, there is no explicit [secondary resources](https://github.com/operator-framework/operator-sdk/blob/master/doc/user-guide.md) representation, which makes it difficult to reconcile with the primary (Assembly) resource. This could be simulated by listening to LM Kafka state change events and building a secondary resource model from those.

# Developing

- [Developing Assembly Operator](./docs/developing.md) - docs for developers to build and install the driver
- [Installing Assembly Operator](./docs/installation.md) - installing Assembly Operator
