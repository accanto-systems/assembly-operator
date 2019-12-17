#!/bin/bash

usage(){
    echo "usage: $(basename "$(test -L "$0" && readlink "$0" || echo "$0")") [OPTIONS] 
  Program to build distribution of assembly-operator

    options:
        -v, --version		version number of operator
        -d, --docker        dev - build a TAR of the docker image, prod - build and push the docker image to the accanto organisation on dockerhub
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

cat >version/version.go <<EOL
package version

var (
  Version = "$version"
)
EOL

echo "Building deployment package assembly-operator-deployment-$version.tgz"
tar -cvzf assembly-operator-deployment-$version.tgz deploy deploy-scripts --transform "s!^deploy-scripts\($\|/\)!assembly-operator-deployment-$version\1!" --transform "s!^deploy\($\|/\)!assembly-operator-deployment-$version\1!"

if [ $docker = "dev" ]; then 
    echo "Building TAR for operator image assembly-operator:$version"
    export GO111MODULE=on
    operator-sdk build assembly-operator:$version
    docker save assembly-operator:$version -o assembly-operator-docker-img-$version.tar
else
    echo "NOTE: Dev Docker Image TAR build not requested"  
fi

if [ $docker = "prod" ]; then 
    echo "Publishing operator image accanto/assembly-operator:$version"
    export GO111MODULE=on
    operator-sdk build accanto/assembly-operator:$version
    docker push accanto/assembly-operator:$version
else
    echo "NOTE: Prod Docker Image publish not requested"  
fi