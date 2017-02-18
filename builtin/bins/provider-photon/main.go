package main

import (
	"github.com/hashicorp/terraform/builtin/providers/photon"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: photon.Provider,
	})
}
