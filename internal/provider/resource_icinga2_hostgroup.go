package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lrsmith/go-icinga2-api/iapi"
)

var (
	_ resource.Resource                = &hostGroupResource{}
	_ resource.ResourceWithConfigure   = &hostGroupResource{}
	_ resource.ResourceWithImportState = &hostGroupResource{}
)

func HostGroup() resource.Resource {
	return &hostGroupResource{}
}

type hostGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	LastUpdated types.String `tfsdk:"last_updated"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Zone        types.String `tfsdk:"zone"`
}

// hostResource defines the resource implementation.
type hostGroupResource struct {
	client *iapi.Server
}

func (r *hostGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hostgroup"
}

func (r *hostGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the HostGroup",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of HostGroup",
			},
			"zone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Zone of HostGroup",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Default: stringdefault.StaticString("master"),
			},
		},
	}
}

func (r *hostGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*iapi.Server)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *iapi.Server, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *hostGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Host group must not exist
	exists, err := HostgroupExists(r.client, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Host Group",
			"Could not verify if host group '"+plan.Name.ValueString()+"' exists: "+err.Error(),
		)
		return
	}

	if exists {
		resp.Diagnostics.AddError(
			"Error creating Host Group",
			"Host group '"+plan.Name.ValueString()+"' already exists",
		)
		return
	}

	hostgroups, err := CreateHostgroup(r.client, plan.Name.ValueString(), plan.DisplayName.ValueString(), plan.Zone.ValueString())

	// Retry on context deadline exceeded
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		checkOperation := func() ([]HostgroupStruct, error) {
			hostgroups, err := GetHostgroup(r.client, plan.Name.ValueString())
			if err != nil {
				return nil, err
			}
			for _, hostgroup := range hostgroups {
				if hostgroup.Name == plan.Name.ValueString() {
					return hostgroups, nil
				}
			}
			return nil, fmt.Errorf("Host group '%s' not found after creation", plan.Name.ValueString())
		}
		hostgroups, err = backoff.RetryWithData(checkOperation, backoff.NewExponentialBackOff())
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Host Group",
			"Could not create host group '"+plan.Name.ValueString()+"': "+err.Error(),
		)
		return
	}

	for _, hostgroup := range hostgroups {
		if hostgroup.Name == plan.Name.ValueString() {
			plan.ID = types.StringValue(hostgroup.Name)
			plan.Name = types.StringValue(hostgroup.Name)
			plan.DisplayName = types.StringValue(hostgroup.Attrs.DisplayName)
			plan.Zone = types.StringValue(hostgroup.Attrs.Zone)
		}
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *hostGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostgroups, err := GetHostgroup(r.client, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Host Group",
			"Could not read host group '"+state.Name.ValueString()+"': "+err.Error(),
		)
		return
	}

	for _, hostgroup := range hostgroups {
		if hostgroup.Name == state.Name.ValueString() {
			state.ID = types.StringValue(hostgroup.Name)
			state.Name = types.StringValue(hostgroup.Name)
			state.DisplayName = types.StringValue(hostgroup.Attrs.DisplayName)
			state.Zone = types.StringValue(hostgroup.Attrs.Zone)
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *hostGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hostGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs := HostgroupAttrs{
		DisplayName: plan.DisplayName.ValueString(),
	}
	_, err := UpdateHostgroup(r.client, plan.ID.ValueString(), attrs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Host Group",
			"Could not update host group '"+plan.Name.ValueString()+"': "+err.Error(),
		)
		return
	}

	hostgroups, err := GetHostgroup(r.client, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Host Group",
			"Could not read host group "+plan.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	for _, hostgroup := range hostgroups {
		if hostgroup.Name == plan.Name.ValueString() {
			plan.ID = types.StringValue(hostgroup.Name)
			plan.Name = types.StringValue(hostgroup.Name)
			plan.DisplayName = types.StringValue(hostgroup.Attrs.DisplayName)
			plan.Zone = types.StringValue(hostgroup.Attrs.Zone)
		}
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set refreshed plan
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *hostGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteHostgroup(state.Name.ValueString())

	// Retry on context deadline exceeded
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		checkOperation := func() error {
			exists, err := HostgroupExists(r.client, state.Name.ValueString())
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("Host group still exists after deletion")
			}
			return nil
		}
		err = backoff.Retry(checkOperation, backoff.NewExponentialBackOff())
	}

	if err != nil {
		var errMsg string
		if err.Error() == "No objects found." {
			errMsg = "Host group '" + state.Name.ValueString() + "' does not exist or insufficient permissions to delete the host group"
		} else {
			errMsg = "Could not delete host group '" + state.Name.ValueString() + "': " + err.Error()
		}
		resp.Diagnostics.AddError(
			"Error deleting Host Group",
			errMsg,
		)
		return
	}
}

func (r *hostGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Host group must exist
	exists, err := HostgroupExists(r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error importing Host Group",
			"Could not verify if host group '"+req.ID+"' exists: "+err.Error(),
		)
		return
	}

	if !exists {
		resp.Diagnostics.AddError(
			"Error importing Host Group",
			"Host group '"+req.ID+"' does not exist",
		)
		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// HostGroup patch
// https://github.com/lrsmith/go-icinga2-api/pull/22

// HostgroupStruct is a struct used to store results from an Icinga2 HostGroup API Call. The content is also used to generate the JSON payload for the CreateHostgroup call.
type HostgroupStruct struct {
	Name  string         `json:"name"`
	Type  string         `json:"type"`
	Attrs HostgroupAttrs `json:"attrs"`
}

// HostgroupAttrs ...
type HostgroupAttrs struct {
	DisplayName string `json:"display_name,omitempty"`
	Zone        string `json:"zone,omitempty"`
}

// From https://raw.githubusercontent.com/lrsmith/go-icinga2-api/refs/heads/master/iapi/hostgroups.go

const hostgroupEndpoint = "/objects/hostgroups"

// GetHostgroup fetches a HostGroup by its name.
func GetHostgroup(server *iapi.Server, name string) ([]HostgroupStruct, error) {
	endpoint := fmt.Sprintf("%v/%v", hostgroupEndpoint, name)
	results, err := server.NewAPIRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Contents of the results is an interface object. Need to convert it to json first.
	jsonStr, err := json.Marshal(results.Results)
	if err != nil {
		return nil, err
	}

	// then the JSON can be pushed into the appropriate struct.
	// Note : Results is a slice so much push into a slice.
	var hostgroups []HostgroupStruct
	if err := json.Unmarshal(jsonStr, &hostgroups); err != nil {
		return nil, err
	}

	if len(hostgroups) == 0 {
		return nil, nil
	}

	if len(hostgroups) != 1 {
		return nil, errors.New("found more than one matching hostgroup")
	}

	return hostgroups, err
}

// CreateHostgroup creates a new HostGroup with its name and display name.
func CreateHostgroup(server *iapi.Server, name, displayName, zone string) ([]HostgroupStruct, error) {
	var newAttrs HostgroupAttrs
	newAttrs.DisplayName = displayName
	newAttrs.Zone = zone

	var newHostgroup HostgroupStruct
	newHostgroup.Name = name
	newHostgroup.Type = "Hostgroup"
	newHostgroup.Attrs = newAttrs

	payloadJSON, err := json.Marshal(newHostgroup)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%v/%v", hostgroupEndpoint, name)
	results, err := server.NewAPIRequest(http.MethodPut, endpoint, payloadJSON)
	if err != nil {
		return nil, err
	}

	if results.Code == http.StatusOK {
		hostgroups, err := GetHostgroup(server, name)
		return hostgroups, err
	}

	return nil, fmt.Errorf("%s", results.ErrorString)
}

type HostgroupUpdateResponse struct {
	Code   float64 `json:"code"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
}

// UpdateHostgroup updates a HostGroup with its attrs.
func UpdateHostgroup(server *iapi.Server, name string, attrs HostgroupAttrs) ([]HostgroupStruct, error) {

	var hostgroup HostgroupStruct
	hostgroup.Attrs = attrs

	body, err := json.Marshal(hostgroup)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%v/%v", hostgroupEndpoint, name)
	r, err := server.NewAPIRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}

	if r.Code != http.StatusOK {
		return nil, fmt.Errorf("expected %d, got %d", http.StatusOK, r.Code)
	}

	jsonResponse, err := json.Marshal(r.Results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the host group response: %v", err)
	}

	var results []HostgroupUpdateResponse
	err = json.Unmarshal(jsonResponse, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall the host group response: %v", err)
	}

	for _, result := range results {
		if result.Code != http.StatusOK {
			return nil, fmt.Errorf("%s", result.Status)
		}
	}

	return GetHostgroup(server, name)
}

// DeleteHostgroup deletes a HostGroup by its name.
func DeleteHostgroup(server *iapi.Server, name string) error {
	endpoint := fmt.Sprintf("%v/%v", hostgroupEndpoint, name)
	results, err := server.NewAPIRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	if results.Code == http.StatusOK {
		return nil
	}

	return fmt.Errorf("%s", results.ErrorString)
}

func HostgroupExists(server *iapi.Server, name string) (bool, error) {
	hostgroups, err := GetHostgroup(server, name)
	if err != nil {
		return false, err
	}

	for _, hostgroup := range hostgroups {
		if hostgroup.Name == name {
			return true, nil
		}
	}

	return false, nil
}
