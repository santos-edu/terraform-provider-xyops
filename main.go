// Package main serves the xyOps Terraform provider.
package main

import (
	"context"
	"log"

	"github.com/santos-edu/terraform-provider-xyops/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// providerAddress is the address used in Terraform configurations:
// terraform { required_providers { xyops = { source = "santos-edu/xyops" } } }
const providerAddress = "registry.terraform.io/santos-edu/xyops"

// version is set at build time via -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: providerAddress,
	})
	if err != nil {
		log.Fatal(err)
	}
}
