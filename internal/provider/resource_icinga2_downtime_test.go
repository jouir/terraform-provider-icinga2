package provider

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lrsmith/go-icinga2-api/iapi"
)

func TestAccCreateBasicDowntime(t *testing.T) {
	hostname := "docker-icinga2"
	Groups := []string{"linux-servers"}
	if os.Getenv("TC_ACC") == "1" {
		skipVerify, err := strconv.ParseBool(os.Getenv("ICINGA2_INSECURE_SKIP_TLS_VERIFY"))
		if err != nil {
			t.Errorf("failed to parse ICINGA2_INSECURE_SKIP_TLS_VERIFY: %v", err)
		}
		server, err := iapi.New(os.Getenv("ICINGA2_API_USER"), os.Getenv("ICINGA2_API_PASSWORD"), os.Getenv("ICINGA2_API_URL"), skipVerify)
		if err != nil {
			t.Errorf("failed to create server: %v", err)
		}
		s, _ := server.GetHost(hostname)
		if len(s) == 0 {
			_, err = server.CreateHost(hostname, "10.0.0.1", "hostalive", nil, nil, Groups)
			if err != nil {
				t.Errorf("failed to create host: %v", err)
			}
		}

		testAccCreateDowntimeBasic := fmt.Sprintf(`
resource "icinga2_downtime" "tf-dt-1" {
  type         = "Host"
  filter       = "host.name==\"docker-icinga2\""
  author       = "terraform"
  comment      = "Initial downtime"
  start_time   = %d
  end_time     = %d
  all_services = false
}`, time.Now().Unix(), time.Now().Unix()+3600)

		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: providerConfig + testAccCreateDowntimeBasic,
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(
							"icinga2_downtime.tf-dt-1",
							tfjsonpath.New("type"),
							knownvalue.StringExact("Host"),
						),
						statecheck.ExpectKnownValue(
							"icinga2_downtime.tf-dt-1",
							tfjsonpath.New("filter"),
							knownvalue.StringExact("host.name==\"docker-icinga2\""),
						),
						statecheck.ExpectKnownValue(
							"icinga2_downtime.tf-dt-1",
							tfjsonpath.New("author"),
							knownvalue.StringExact("terraform"),
						),
						statecheck.ExpectKnownValue(
							"icinga2_downtime.tf-dt-1",
							tfjsonpath.New("comment"),
							knownvalue.StringExact("Initial downtime"),
						),
					},
				},
			},
		})

		err = server.DeleteHost(hostname)
		if err != nil {
			t.Errorf("failed to delete host: %v", err)
		}
	}
}
