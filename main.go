package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terra-farm/terraform-provider-virtualbox/internal/provider"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "run the provider in debug mode")
	flag.Parse()

	ctx := context.Background()

	// SDK v2 provider (existing resources)
	sdkProvider := func() *schema.Provider {
		return provider.New()
	}

	// Framework provider (new resources will be added here)
	frameworkProvider := providerserver.NewProtocol5(
		provider.NewFrameworkProvider(version)(),
	)

	// Mux both providers together
	muxServer, err := tf5muxserver.NewMuxServer(ctx,
		func() tfprotov5.ProviderServer {
			return sdkProvider().GRPCProvider()
		},
		frameworkProvider,
	)
	if err != nil {
		log.Fatal(err)
	}

	var serveOpts []tf5server.ServeOpt
	if *debug {
		serveOpts = append(serveOpts, tf5server.WithManagedDebug())
	}

	err = tf5server.Serve(
		"registry.terraform.io/eran132/virtualbox",
		muxServer.ProviderServer,
		serveOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}
}
