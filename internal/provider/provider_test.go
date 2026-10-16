package provider

import (
	"os"
	"testing"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"xyops": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("XYOPS_ENDPOINT") == "" {
		t.Skip("XYOPS_ENDPOINT not set; skipping acceptance test")
	}
	if os.Getenv("XYOPS_API_KEY") == "" {
		t.Skip("XYOPS_API_KEY not set; skipping acceptance test")
	}
}

func providerConfig() string {
	return `provider "xyops" {}`
}

func testAccClient(t *testing.T) *xyops.Client {
	endpoint := os.Getenv("XYOPS_ENDPOINT")
	apiKey := os.Getenv("XYOPS_API_KEY")
	client, err := xyops.NewClient(endpoint, apiKey)
	if err != nil {
		t.Fatalf("failed to create xyOps test client: %s", err)
	}
	return client
}

func testAccAlertID() string    { return "tfaccalert" }
func testAccBucketID() string   { return "tfaccbucket" }
func testAccEventID() string    { return "tfaccevent" }
func testAccWorkflowID() string { return "tfaccworkflow" }
func testAccMonitorID() string  { return "tfaccmonitor" }
func testAccRoleID() string     { return "tfaccrole" }
func testAccAPIKeyID() string   { return "tfaccapikey" }
