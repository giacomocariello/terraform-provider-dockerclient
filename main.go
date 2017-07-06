package main

import (
	"fmt"
	"log"
	"os"

	terraform "github.com/hashicorp/terraform/plugin"

	"github.com/giacomocariello/terraform-provider-dockerimage/provider"
)

func main() {
	log.SetFlags(log.Lshortfile)
	log.SetPrefix(fmt.Sprintf("pid-%d-", os.Getpid()))

	terraform.Serve(&terraform.ServeOpts{
		ProviderFunc: provider.Provider,
	})
}
