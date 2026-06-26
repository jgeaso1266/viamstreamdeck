package main

import (
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"

	viamstreamdeck "github.com/viam-modules/chess-streamdeck"
)

func main() {

	arr := []resource.APIModel{
		{generic.API, viamstreamdeck.PickupModel},
		{generic.API, viamstreamdeck.ModelAny},
	}

	for _, m := range viamstreamdeck.Models {
		arr = append(arr, resource.APIModel{generic.API, m.Model})
	}

	module.ModularMain(arr...)
}
