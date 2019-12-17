#!/bin/bash

set -e

usage(){
    echo "usage: $(basename "$(test -L "$0" && readlink "$0" || echo "$0")") [OPTIONS] 
  Program to build distribution of assembly-operator

    options:
        -v, --version   REQUIRED: version number of operator
        -d, --docker    OPTIONAL: dev - build a TAR of the docker image, prod - build and push the docker image to the accanto organisation on Dockerhub
    "
}

version=""
docker=""

while [ "$1" != "" ]; do
    case $1 in
        -v | --version )          
            shift
            version=$1
            ;;
        -d | --docker )      
            shift
            docker=$1
            ;;
        -h | --help )               
            usage
            exit
            ;;
        * )                         
            usage
            exit 1
    esac
    shift
done

if [[ $version ]]; then
    echo "Version: $version"
else
    echo "Aborted: No version (-v, --version) was set"
    exit 1;
fi

echo "Updating version.go"
cat >version/version.go <<EOL
package version

var (
  Version = "$version"
)
EOL

echo "Preparing pkgdist directory"
rm -rf pkgdist
mkdir pkgdist

echo "Copying deployment sources to pkgdist directory"
cp -r deploy/* pkgdist
cp -r docs/dist-docs/* pkgdist
cp -r dist-scripts/* pkgdist

if [[ $docker = "dev" ]]; then 
    echo "Setting image version in operator.yaml to assembly-operator:$version"
    sed -i "s%^\(\s*image\s*:\s*\).*%\1assembly-operator:$version%" pkgdist/operator.yaml   
else
    echo "Setting image version in operator.yaml to accanto/assembly-operator:$version"
    sed -i "s%^\(\s*image\s*:\s*\).*%\1accanto/assembly-operator:$version%" pkgdist/operator.yaml   
fi

echo "Building deployment package assembly-operator-deployment-$version.tgz"
tar -cvzf assembly-operator-deployment-$version.tgz pkgdist --transform "s!^pkgdist\($\|/\)!assembly-operator-deployment-$version\1!"

if [[ $docker = "dev" ]]; then 
    echo "Building TAR for operator image assembly-operator:$version"
    export GO111MODULE=on
    operator-sdk build assembly-operator:$version
    docker save assembly-operator:$version -o assembly-operator-docker-img-$version.tar
else
    echo "NOTE: Dev Docker Image TAR build not requested"  
fi
if [[ $docker = "prod" ]]; then 
    echo "Publishing operator image accanto/assembly-operator:$version"
    export GO111MODULE=on
    operator-sdk build accanto/assembly-operator:$version
    docker push accanto/assembly-operator:$version
else
    echo "NOTE: Prod Docker Image publish not requested"  
fi

echo "Removing pkgdist directory"
rm -rf pkgdist