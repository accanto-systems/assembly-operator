#!/bin/bash

set -e

namespaceOpt=""
if [ -z "$1" ]
then
      namespaceOpt="--namespace=$1"
fi

kubectl delete crds assemblys.com.accantosystems.stratoss $namespaceOpt
kubectl delete cm assembly-operator-config $namespaceOpt
kubectl delete deployment assembly-operator $namespaceOpt
kubectl delete role assembly-operator $namespaceOpt
kubectl delete rolebinding assembly-operator $namespaceOpt
kubectl delete serviceaccount assembly-operator $namespaceOpt