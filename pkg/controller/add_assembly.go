package controller

import (
	"github.com/orgs/accanto-systems/assembly-operator/pkg/controller/assembly"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, assembly.Add)
}
