#!/bin/bash

set -e

usage(){
    echo "usage: $(basename "$(test -L "$0" && readlink "$0" || echo "$0")") [OPTIONS] 
  Program to build distribution of assembly-operator

    options:
        -v, --version   REQUIRED: version number of operator
    "
}

version=""

while [ "$1" != "" ]; do
    case $1 in
        -v | --version )          
            shift
            version=$1
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


echo "Scp Deploy Files"
scp assembly-operator-deployment-$version.tgz accanto@10.220.217.247:/home/accanto/assembly-operator-install
echo "Tagging image"
docker image tag assembly-operator:$version 10.220.216.164:5000/assembly-operator:$version
echo "Pushing image"
docker image push 10.220.216.164:5000/assembly-operator:$version
