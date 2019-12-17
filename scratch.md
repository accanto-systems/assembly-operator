cd ~/src
operator-sdk new assembly-operator

operator-sdk add api --api-version=com.accantosystems.stratoss/v1alpha1 --kind=Assembly
operator-sdk add controller --api-version=com.accantosystems.stratoss/v1alpha1 --kind=Assembly
