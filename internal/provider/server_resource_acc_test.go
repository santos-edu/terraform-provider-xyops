package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
)

const (
	testAccServerID      = "local"
	testAccServerAddress = "xyops_server.test"
)

// Every test in this file adopts the real local server, so each one restores it
// to enabled = true with empty notes on the way out, failing or not.
func testAccServerRestore(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
	testAccPreCheck(t)
	t.Cleanup(func() {
		payload := map[string]any{"id": testAccServerID, "enabled": true, "notes": ""}
		if err := testAccClient(t).Call("update_server", payload, nil); err != nil {
			t.Logf("failed to restore xyOps server %s: %s", testAccServerID, err)
		}
	})
}

func testAccServerAPI(t *testing.T) (serverPayload, error) {
	t.Helper()
	var result serverResponse
	if err := testAccClient(t).Call("get_server", map[string]any{"id": testAccServerID}, &result); err != nil {
		return serverPayload{}, err
	}
	return result.Server, nil
}

func testAccServerAPIValues(t *testing.T, enabled bool, notes string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		server, err := testAccServerAPI(t)
		if err != nil {
			return fmt.Errorf("reading xyOps server %s: %w", testAccServerID, err)
		}
		if server.Enabled != enabled || server.Notes != notes {
			return fmt.Errorf("xyOps server %s reports enabled = %t, notes = %q; want enabled = %t, notes = %q", testAccServerID, server.Enabled, server.Notes, enabled, notes)
		}
		return nil
	}
}

// testAccServerUntouched proves that adopting without enabled and notes writes
// nothing: update_server always advances modified, so an unchanged timestamp
// means no write reached the server.
func testAccServerUntouched(t *testing.T, before *serverPayload) resource.TestCheckFunc {
	return func(*terraform.State) error {
		after, err := testAccServerAPI(t)
		if err != nil {
			return fmt.Errorf("reading xyOps server %s: %w", testAccServerID, err)
		}
		if after.Enabled != before.Enabled || after.Notes != before.Notes {
			return fmt.Errorf("adopting with enabled and notes omitted changed the server: before enabled = %t, notes = %q; after enabled = %t, notes = %q", before.Enabled, before.Notes, after.Enabled, after.Notes)
		}
		if after.Modified != before.Modified {
			return fmt.Errorf("adopting with enabled and notes omitted still wrote to the server: modified went from %d to %d", before.Modified, after.Modified)
		}
		return nil
	}
}

func testAccServerStateMatchesAPI(t *testing.T) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		server, err := testAccServerAPI(t)
		if err != nil {
			return fmt.Errorf("reading xyOps server %s: %w", testAccServerID, err)
		}
		resourceState, ok := state.RootModule().Resources[testAccServerAddress]
		if !ok {
			return fmt.Errorf("%s is missing from the state", testAccServerAddress)
		}
		want := map[string]string{
			"id":         server.ID,
			"enabled":    strconv.FormatBool(server.Enabled),
			"notes":      server.Notes,
			"hostname":   server.Hostname,
			"ip":         server.IP,
			"auto_group": strconv.FormatBool(server.AutoGroup),
			"created":    strconv.FormatInt(server.Created, 10),
			"modified":   strconv.FormatInt(server.Modified, 10),
			"groups.#":   strconv.Itoa(len(server.Groups)),
		}
		for attribute, value := range want {
			if got := resourceState.Primary.Attributes[attribute]; got != value {
				return fmt.Errorf("terraform state %s = %q, xyOps reports %q", attribute, got, value)
			}
		}
		for i, group := range server.Groups {
			if got := resourceState.Primary.Attributes[fmt.Sprintf("groups.%d", i)]; got != group {
				return fmt.Errorf("terraform state groups.%d = %q, xyOps reports %q", i, got, group)
			}
		}
		return nil
	}
}

func testAccServerStillReadable(t *testing.T) resource.TestCheckFunc {
	return func(*terraform.State) error {
		server, err := testAccServerAPI(t)
		if err != nil {
			return fmt.Errorf("xyOps server %s is not readable after destroy: %w", testAccServerID, err)
		}
		if server.ID != testAccServerID {
			return fmt.Errorf("xyOps reports server %q after destroy, want %q", server.ID, testAccServerID)
		}
		return nil
	}
}

// testAccServerStillWritable proves the destroy did not evict the server from
// the live registry. delete_server answers code 0 without deleting anything, so
// its damage does not show up in a read: the record stays readable with the
// same settings, and only the writes stop showing up. Writing and reading back
// is the difference between "the settings did not change" and "the server is
// still manageable".
func testAccServerStillWritable(t *testing.T, enabled bool) resource.TestCheckFunc {
	const tolerance = 15 * time.Second
	return func(*terraform.State) error {
		payload := map[string]any{"id": testAccServerID, "enabled": enabled}
		if err := testAccClient(t).Call("update_server", payload, nil); err != nil {
			return fmt.Errorf("writing to xyOps server %s after destroy failed: %w", testAccServerID, err)
		}
		deadline := time.Now().Add(tolerance)
		for {
			server, err := testAccServerAPI(t)
			if err != nil {
				return fmt.Errorf("reading xyOps server %s after writing to it: %w", testAccServerID, err)
			}
			if server.Enabled == enabled {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("xyOps server %s accepted a write of enabled = %t but %s later a read still reports enabled = %t: the server was evicted from the live registry and is no longer manageable", testAccServerID, enabled, tolerance, server.Enabled)
			}
			time.Sleep(time.Second)
		}
	}
}

func testAccServerAbsentFromState() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if _, ok := state.RootModule().Resources[testAccServerAddress]; ok {
			return fmt.Errorf("%s is still in the state after destroy", testAccServerAddress)
		}
		return nil
	}
}

func TestAccServerResource_adopt(t *testing.T) {
	testAccServerRestore(t)
	config := providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = false
  notes   = "tf-acc-server-adopt"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "id", testAccServerID),
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "false"),
					resource.TestCheckResourceAttr(testAccServerAddress, "notes", "tf-acc-server-adopt"),
					testAccServerAPIValues(t, false, "tf-acc-server-adopt"),
					testAccServerStateMatchesAPI(t),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServerResource_update(t *testing.T) {
	testAccServerRestore(t)
	updatedConfig := providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = true
  notes   = "tf-acc-server-v2"
}
`
	clearedConfig := providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = true
  notes   = ""
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = false
  notes   = "tf-acc-server-v1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "false"),
					resource.TestCheckResourceAttr(testAccServerAddress, "notes", "tf-acc-server-v1"),
					testAccServerAPIValues(t, false, "tf-acc-server-v1"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "true"),
					resource.TestCheckResourceAttr(testAccServerAddress, "notes", "tf-acc-server-v2"),
					testAccServerAPIValues(t, true, "tf-acc-server-v2"),
					testAccServerStateMatchesAPI(t),
				),
			},
			{
				Config: updatedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: clearedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "true"),
					resource.TestCheckResourceAttr(testAccServerAddress, "notes", ""),
					testAccServerAPIValues(t, true, ""),
					testAccServerStateMatchesAPI(t),
				),
			},
			{
				Config: clearedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServerResource_drift(t *testing.T) {
	testAccServerRestore(t)
	config := providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = false
  notes   = "tf-acc-server-drift"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "false"),
				),
			},
			{
				PreConfig: func() {
					payload := map[string]any{"id": testAccServerID, "enabled": true}
					if err := testAccClient(t).Call("update_server", payload, nil); err != nil {
						t.Fatalf("failed to drift xyOps server %s: %s", testAccServerID, err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "false"),
					resource.TestCheckResourceAttr(testAccServerAddress, "notes", "tf-acc-server-drift"),
					testAccServerAPIValues(t, false, "tf-acc-server-drift"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServerResource_import(t *testing.T) {
	testAccServerRestore(t)
	adoptedConfig := providerConfig() + `
resource "xyops_server" "test" {
  id = "local"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = false
  notes   = "tf-acc-server-import"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "enabled", "false"),
					testAccServerAPIValues(t, false, "tf-acc-server-import"),
				),
			},
			{
				ResourceName:      testAccServerAddress,
				ImportState:       true,
				ImportStateId:     testAccServerID,
				ImportStateVerify: true,
			},
			{
				Config: adoptedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccServerResource_omittedOptionals(t *testing.T) {
	testAccServerRestore(t)
	config := providerConfig() + `
resource "xyops_server" "test" {
  id = "local"
}
`
	var before serverPayload
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					server, err := testAccServerAPI(t)
					if err != nil {
						t.Fatalf("failed to read xyOps server %s before adopting it: %s", testAccServerID, err)
					}
					before = server
				},
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccServerAddress, "id", testAccServerID),
					resource.TestCheckResourceAttrSet(testAccServerAddress, "enabled"),
					resource.TestCheckResourceAttrSet(testAccServerAddress, "hostname"),
					testAccServerUntouched(t, &before),
					testAccServerStateMatchesAPI(t),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// An id that is still unknown cannot be adopted: there is no such thing as
// looking up the empty id and reporting whatever comes back as this server, so
// Create has to stop and say what the operator has to fix.
func TestServerResourceCreate_unknownID(t *testing.T) {
	ctx := context.Background()
	schemaResp := &fwresource.SchemaResponse{}
	(&serverResource{}).Schema(ctx, fwresource.SchemaRequest{}, schemaResp)

	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	plan := tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"enabled":    tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
		"notes":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"hostname":   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"ip":         tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"auto_group": tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
		"groups":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue),
		"created":    tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"modified":   tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	})

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":0,"server":{"id":"","enabled":true,"hostname":"","ip":"","autoGroup":true,"groups":[],"created":1,"modified":2}}`)
	}))
	defer server.Close()
	client, err := xyops.NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("failed to build the xyOps client: %s", err)
	}

	resp := &fwresource.CreateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)},
	}
	(&serverResource{client: client}).Create(ctx, fwresource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Create with an unknown id reported no error, want one")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Unknown xyOps server id" {
		t.Errorf("diagnostic summary is %q, want %q", summary, "Unknown xyOps server id")
	}
	if len(calls) != 0 {
		t.Errorf("Create called %v, want no call at all: an unknown id is not a server to look up", calls)
	}
}

// Terraform resolves a reference to another resource before it calls the
// provider, so an acceptance test cannot hand Update a value that is still
// unknown. Driving Update directly is the only way to cover the case it creates:
// a config that asks for a value Terraform cannot know at plan time arrives
// here unknown, and if the server already carries the wanted values the write
// is skipped and those unknowns have to fall back to the previous state, which
// is what keeps the apply from failing with "not wholly known".
func TestServerResourceUpdate_unknownPlanValues(t *testing.T) {
	ctx := context.Background()
	schemaResp := &fwresource.SchemaResponse{}
	(&serverResource{}).Schema(ctx, fwresource.SchemaRequest{}, schemaResp)

	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	priorState := tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "local"),
		"enabled":    tftypes.NewValue(tftypes.Bool, true),
		"notes":      tftypes.NewValue(tftypes.String, "tf-acc-server-unknown"),
		"hostname":   tftypes.NewValue(tftypes.String, "xyops-dev"),
		"ip":         tftypes.NewValue(tftypes.String, "127.0.0.1"),
		"auto_group": tftypes.NewValue(tftypes.Bool, true),
		"groups":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "xyops")}),
		"created":    tftypes.NewValue(tftypes.Number, 1),
		"modified":   tftypes.NewValue(tftypes.Number, 2),
	})
	unknownPlan := tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "local"),
		"enabled":    tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
		"notes":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"hostname":   tftypes.NewValue(tftypes.String, "xyops-dev"),
		"ip":         tftypes.NewValue(tftypes.String, "127.0.0.1"),
		"auto_group": tftypes.NewValue(tftypes.Bool, true),
		"groups":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "xyops")}),
		"created":    tftypes.NewValue(tftypes.Number, 1),
		"modified":   tftypes.NewValue(tftypes.Number, 2),
	})

	var prior serverModel
	if diags := (tfsdk.State{Schema: schemaResp.Schema, Raw: priorState}).Get(ctx, &prior); diags.HasError() {
		t.Fatalf("failed to decode the prior state: %s", diags)
	}

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":0,"server":{"id":"local","enabled":true,"notes":"tf-acc-server-unknown","hostname":"xyops-dev","ip":"127.0.0.1","autoGroup":true,"groups":["xyops"],"created":1,"modified":2}}`)
	}))
	defer server.Close()
	client, err := xyops.NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("failed to build the xyOps client: %s", err)
	}

	resp := &fwresource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorState},
	}
	(&serverResource{client: client}).Update(ctx, fwresource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: unknownPlan},
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorState},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update with unknown plan values failed: %s", resp.Diagnostics)
	}
	if len(calls) != 0 {
		t.Errorf("Update called %v, want no call at all: the server already carries the wanted values", calls)
	}

	var got serverModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read the resulting state: %s", diags)
	}
	if got.Enabled.IsUnknown() || got.Notes.IsUnknown() {
		t.Fatalf("the resulting state is not wholly known: enabled unknown = %t, notes unknown = %t", got.Enabled.IsUnknown(), got.Notes.IsUnknown())
	}
	if got.Enabled.ValueBool() != prior.Enabled.ValueBool() || got.Notes.ValueString() != prior.Notes.ValueString() {
		t.Errorf("state kept enabled = %t, notes = %q; want the previous enabled = %t, notes = %q", got.Enabled.ValueBool(), got.Notes.ValueString(), prior.Enabled.ValueBool(), prior.Notes.ValueString())
	}
	if got.Modified.ValueInt64() != prior.Modified.ValueInt64() {
		t.Errorf("modified = %d, want %d: an unused write must not pretend the server changed", got.Modified.ValueInt64(), prior.Modified.ValueInt64())
	}
}

func TestAccServerResource_serverDoesNotExist(t *testing.T) {
	testAccServerRestore(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_server" "test" {
  id = "tfaccnosuchserver"
}
`,
				ExpectError: regexp.MustCompile("does not exist in xyOps"),
			},
		},
	})
}

func TestAccServerResource_destroyLeavesServer(t *testing.T) {
	testAccServerRestore(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccServerStillReadable(t),
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_server" "test" {
  id      = "local"
  enabled = false
  notes   = "tf-acc-server-destroy"
}
`,
				Check: testAccServerAPIValues(t, false, "tf-acc-server-destroy"),
			},
			{
				Config: providerConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccServerAPIValues(t, false, "tf-acc-server-destroy"),
					testAccServerAbsentFromState(),
					testAccServerStillWritable(t, true),
				),
			},
		},
	})
}

// get_server refuses an id whose shape it will not even look up with code
// "api", which is not the same as an id it looked up and did not find. The two
// need different reports: one is a typo to fix, the other a server that has not
// registered yet.
func TestServerResourceLookupError_malformedID(t *testing.T) {
	summary, detail := serverLookupError("Bad ID", &xyops.APIError{Code: "api", Description: "Malformed parameter: id"})
	if summary != "Invalid xyOps server id" {
		t.Errorf("malformed id summary is %q, want %q", summary, "Invalid xyOps server id")
	}
	if !regexp.MustCompile("lowercase letters, digits and underscores").MatchString(detail) {
		t.Errorf("malformed id detail is %q, want it to explain the accepted charset", detail)
	}
	if summary, _ := serverLookupError("naoexiste", &xyops.APIError{Code: "notfound", Description: "Not found"}); summary != "xyOps server does not exist" {
		t.Errorf("notfound summary is %q, want %q", summary, "xyOps server does not exist")
	}
}

// P2.16 against the real conductor: a local server and a remote satellite
// managed together by Terraform, with the satellite agent stopped and started
// under the harness. Needs the xysat container from the compose profile
// "satellite" (docker compose --profile satellite up -d xysat-remote) with
// XYOPS_SATELLITE_ID set to its server id. Restores the satellite state and
// restarts the container on the way out, failing or not.
func TestAccServerResource_twoServersSatelliteOffline(t *testing.T) {
	satelliteID := os.Getenv("XYOPS_SATELLITE_ID")
	satelliteContainer := os.Getenv("XYOPS_SATELLITE_CONTAINER")
	if satelliteID == "" || satelliteContainer == "" {
		t.Skip("XYOPS_SATELLITE_ID and XYOPS_SATELLITE_CONTAINER not set; start the xysat container from the compose profile 'satellite' to run this test")
	}
	t.Cleanup(func() {
		payload := map[string]any{"id": satelliteID, "enabled": true, "notes": ""}
		if err := testAccClient(t).Call("update_server", payload, nil); err != nil {
			t.Logf("failed to restore xyOps satellite %s: %s", satelliteID, err)
		}
		if err := exec.Command("docker", "start", satelliteContainer).Run(); err != nil {
			t.Errorf("start satellite container: %s", err)
			return
		}
	})

	satelliteAddress := "xyops_server.satellite"
	baseConfig := providerConfig() + fmt.Sprintf(`
resource "xyops_server" "local" {
  id      = %q
  enabled = true
  notes   = "tf-acc-two-servers"
}
resource "xyops_server" "satellite" {
  id      = %q
  enabled = true
  notes   = "tf-acc-two-servers"
}
`, testAccServerID, satelliteID)
	updatedConfig := providerConfig() + fmt.Sprintf(`
resource "xyops_server" "local" {
  id      = %q
  enabled = true
  notes   = "tf-acc-two-servers-v2"
}
resource "xyops_server" "satellite" {
  id      = %q
  enabled = true
  notes   = "tf-acc-two-servers-v2"
}
`, testAccServerID, satelliteID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if _, err := testAccServerAPIFor(t, satelliteID); err != nil {
				return fmt.Errorf("xyOps satellite %s is not readable after destroy: %w", satelliteID, err)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: baseConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_server.local", "notes", "tf-acc-two-servers"),
					resource.TestCheckResourceAttr(satelliteAddress, "id", satelliteID),
					resource.TestCheckResourceAttr(satelliteAddress, "notes", "tf-acc-two-servers"),
					testAccServerOnline(t, satelliteID, true),
				),
			},
			{
				PreConfig: func() {
					if err := exec.Command("docker", "stop", satelliteContainer).Run(); err != nil {
						t.Fatalf("failed to stop the xysat container %s: %s", satelliteContainer, err)
					}
					testAccServerWaitOnline(t, satelliteID, false)
				},
				Config: baseConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				// The half of the state beyond these plan checks is what
				// matters: get_server answers code 0 with online = false and
				// the persisted server, so a satellite offline must not abort
				// the plan or evict the resource from the state.
			},
			{
				PreConfig: func() {
					if err := exec.Command("docker", "start", satelliteContainer).Run(); err != nil {
						t.Fatalf("failed to start the xysat container %s: %s", satelliteContainer, err)
					}
					testAccServerWaitOnline(t, satelliteID, true)
				},
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_server.local", "notes", "tf-acc-two-servers-v2"),
					resource.TestCheckResourceAttr(satelliteAddress, "notes", "tf-acc-two-servers-v2"),
					testAccServerAPIValuesFor(t, testAccServerID, true, "tf-acc-two-servers-v2"),
					testAccServerAPIValuesFor(t, satelliteID, true, "tf-acc-two-servers-v2"),
				),
			},
			{
				Config: updatedConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccServerAPIFor(t *testing.T, id string) (serverPayload, error) {
	t.Helper()
	var result serverResponse
	if err := testAccClient(t).Call("get_server", map[string]any{"id": id}, &result); err != nil {
		return serverPayload{}, err
	}
	return result.Server, nil
}

func testAccServerAPIValuesFor(t *testing.T, id string, enabled bool, notes string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		server, err := testAccServerAPIFor(t, id)
		if err != nil {
			return fmt.Errorf("reading xyOps server %s: %w", id, err)
		}
		if server.Enabled != enabled || server.Notes != notes {
			return fmt.Errorf("xyOps server %s reports enabled = %t, notes = %q; want enabled = %t, notes = %q", id, server.Enabled, server.Notes, enabled, notes)
		}
		return nil
	}
}

// testAccServerWaitOnline polls get_server until the cluster field `online`
// matches want. The conductor reports an abruptly disconnected xySat within
// seconds, so the deadline is generous for slow machines only.
func testAccServerWaitOnline(t *testing.T, id string, wantOnline bool) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		var probe struct {
			Code   any  `json:"code"`
			Online bool `json:"online"`
		}
		if err := testAccClient(t).Call("get_server", map[string]any{"id": id}, &probe); err != nil {
			t.Fatalf("failed to read xyOps server %s: %s", id, err)
		}
		if probe.Online == wantOnline {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("xyOps server %s is still online = %t after %s, want %t", id, probe.Online, time.Minute, wantOnline)
		}
		time.Sleep(time.Second)
	}
}

func testAccServerOnline(t *testing.T, id string, wantOnline bool) resource.TestCheckFunc {
	return func(*terraform.State) error {
		var probe struct {
			Code   any  `json:"code"`
			Online bool `json:"online"`
		}
		if err := testAccClient(t).Call("get_server", map[string]any{"id": id}, &probe); err != nil {
			return fmt.Errorf("reading xyOps server %s: %w", id, err)
		}
		if probe.Online != wantOnline {
			return fmt.Errorf("xyOps server %s reports online = %t, want %t", id, probe.Online, wantOnline)
		}
		return nil
	}
}

func serverTypeValue(t *testing.T, objectType tftypes.Object, server serverPayload) tftypes.Value {
	t.Helper()
	groupValues := make([]tftypes.Value, len(server.Groups))
	for i, group := range server.Groups {
		groupValues[i] = tftypes.NewValue(tftypes.String, group)
	}
	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, server.ID),
		"enabled":    tftypes.NewValue(tftypes.Bool, server.Enabled),
		"notes":      tftypes.NewValue(tftypes.String, server.Notes),
		"hostname":   tftypes.NewValue(tftypes.String, server.Hostname),
		"ip":         tftypes.NewValue(tftypes.String, server.IP),
		"auto_group": tftypes.NewValue(tftypes.Bool, server.AutoGroup),
		"groups":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, groupValues),
		"created":    tftypes.NewValue(tftypes.Number, server.Created),
		"modified":   tftypes.NewValue(tftypes.Number, server.Modified),
	})
}

// Read has to survive a server whose agent went offline. get_server answers code
// 0 with online = false and the persisted server, never code "server", so the
// resource stays manageable while the agent is down. An id it cannot find is the
// only "gone" signal, and anything else, code "server" included, is a real
// failure.
func TestServerResourceRead(t *testing.T) {
	ctx := context.Background()
	schemaResp := &fwresource.SchemaResponse{}
	(&serverResource{}).Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	prior := serverPayload{
		ID:        "smoffline",
		Hostname:  "sat-1",
		Enabled:   true,
		Notes:     "tf-acc-offline",
		Groups:    []string{"main"},
		AutoGroup: true,
		IP:        "10.0.0.9",
		Created:   1,
		Modified:  2,
	}
	const payload = `{"id":"smoffline","hostname":"sat-1","enabled":true,"notes":"tf-acc-offline","groups":["main"],"autoGroup":true,"ip":"10.0.0.9","created":1,"modified":2}`

	cases := []struct {
		name        string
		body        string
		wantSummary string
		wantRemoved bool
	}{
		{
			name: "offline agent keeps the resource",
			body: `{"code":0,"server":` + payload + `,"online":false}`,
		},
		{
			name: "online agent keeps the resource",
			body: `{"code":0,"server":` + payload + `,"online":true}`,
		},
		{
			name:        "missing server removes the resource",
			body:        `{"code":"notfound","description":"Failed to locate server: smoffline"}`,
			wantRemoved: true,
		},
		{
			name:        "code server is still an error",
			body:        `{"code":"server","description":"Server has gone offline: smoffline"}`,
			wantSummary: "Read xyOps server failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := xyops.NewClient(server.URL, "test-key")
			if err != nil {
				t.Fatalf("failed to build the xyOps client: %s", err)
			}

			priorRaw := serverTypeValue(t, objectType, prior)
			resp := &fwresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorRaw}}
			(&serverResource{client: client}).Read(ctx, fwresource.ReadRequest{
				State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorRaw},
			}, resp)

			if tc.wantSummary != "" {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("Read reported no error, want %q", tc.wantSummary)
				}
				if got := resp.Diagnostics.Errors()[0].Summary(); got != tc.wantSummary {
					t.Errorf("diagnostic summary is %q, want %q", got, tc.wantSummary)
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read failed: %s", resp.Diagnostics)
			}
			if tc.wantRemoved {
				if !resp.State.Raw.IsNull() {
					t.Errorf("state is not null after removing the resource: %v", resp.State.Raw)
				}
				return
			}
			var got serverModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() {
				t.Fatalf("failed to read the resulting state: %s", diags)
			}
			if got.ID.ValueString() != prior.ID {
				t.Errorf("state id = %q, want %q", got.ID.ValueString(), prior.ID)
			}
			if got.Enabled.ValueBool() != prior.Enabled {
				t.Errorf("state enabled = %t, want %t", got.Enabled.ValueBool(), prior.Enabled)
			}
		})
	}
}

// A satellite is just a server that registered itself, so xyops_server adopts it
// the same way and the provider has no satellite-specific code path to test.
// This needs a real remote xySat, so it only runs when XYOPS_SATELLITE_ID points
// at one. CI starts a xysat container and sets the variable.
func TestAccServerResource_adoptSatellite(t *testing.T) {
	id := os.Getenv("XYOPS_SATELLITE_ID")
	if id == "" {
		t.Skip("XYOPS_SATELLITE_ID not set; start a remote xySat and set it to run this test")
	}
	const address = "xyops_server.sat"
	config := providerConfig() + fmt.Sprintf(`
resource "xyops_server" "sat" {
  id      = %q
  enabled = true
  notes   = "tf-acc-satellite"
}
`, id)
	t.Cleanup(func() {
		payload := map[string]any{"id": id, "enabled": true, "notes": ""}
		if err := testAccClient(t).Call("update_server", payload, nil); err != nil {
			t.Logf("failed to restore xyOps satellite %s: %s", id, err)
		}
	})
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			var result serverResponse
			if err := testAccClient(t).Call("get_server", map[string]any{"id": id}, &result); err != nil {
				return fmt.Errorf("xyOps satellite %s is not readable after destroy: %w", id, err)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "id", id),
					resource.TestCheckResourceAttr(address, "enabled", "true"),
					resource.TestCheckResourceAttr(address, "notes", "tf-acc-satellite"),
					resource.TestCheckResourceAttrSet(address, "hostname"),
					resource.TestCheckResourceAttrSet(address, "groups.#"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      address,
				ImportState:       true,
				ImportStateId:     id,
				ImportStateVerify: true,
			},
		},
	})
}
