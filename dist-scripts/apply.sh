#!/bin/bash

set -e

namespaceOpt=""
if [ -z "$1" ]
then
      namespaceOpt="--namespace=$1"
fi

kubectl apply -f service_account.yaml $namespaceOpt
kubectl apply -f role.yaml $namespaceOpt
kubectl apply -f role_binding.yaml $namespaceOpt
kubectl apply -f crds/stratoss.accantosystems.com_assemblies_crd.yaml $namespaceOpt
kubectl apply -f operator.yaml $namespaceOpt
