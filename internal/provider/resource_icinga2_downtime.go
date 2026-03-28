package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lrsmith/go-icinga2-api/iapi"
)

var (
	_ resource.Resource              = &downtimeResource{}
	_ resource.ResourceWithConfigure = &downtimeResource{}
)

func Downtime() resource.Resource {
	return &downtimeResource{}
}

type downtimeResourceModel struct {
	Names        types.List   `tfsdk:"names"`
	LastUpdated  types.String `tfsdk:"last_updated"`
	Type         types.String `tfsdk:"type"`
	Filter       types.String `tfsdk:"filter"`
	Author       types.String `tfsdk:"author"`
	Comment      types.String `tfsdk:"comment"`
	StartTime    types.Int64  `tfsdk:"start_time"`
	EndTime      types.Int64  `tfsdk:"end_time"`
	Fixed        types.Bool   `tfsdk:"fixed"`
	Duration     types.Int64  `tfsdk:"duration"`
	AllServices  types.Bool   `tfsdk:"all_services"`
	TriggerName  types.String `tfsdk:"trigger_name"`
	ChildOptions types.String `tfsdk:"child_options"`
}

// hostResource defines the resource implementation.
type downtimeResource struct {
	client *iapi.Server
}

func (r *downtimeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_downtime"
}

// https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#icinga2-api-actions-schedule-downtime
func (r *downtimeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"names": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Type of downtime (Host or Service).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"filter": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "[Filter](https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#icinga2-api-filters) to apply the downtime.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"author": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the author.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comment": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Comment text.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"start_time": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Unix timestamp marking the beginning of the downtime.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"end_time": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Unix timestamp marking the end of the downtime.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"fixed": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Defaults to true. If true, the downtime is fixed otherwise flexible",
				Default:             booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"duration": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Required for flexible downtimes. Duration of the downtime in seconds if fixed is set to false.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"all_services": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional for host downtimes. Sets downtime for all services for the matched host objects. If child_options are set, all child hosts and their services will schedule a downtime too. Defaults to false.",
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"trigger_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Sets the trigger for a triggered downtime.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"child_options": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Schedule child downtimes. DowntimeNoChildren does not do anything, DowntimeTriggeredChildren schedules child downtimes triggered by this downtime, DowntimeNonTriggeredChildren schedules non-triggered downtimes. Defaults to DowntimeNoChildren.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *downtimeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *downtimeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan downtimeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	names, err := CreateDowntime(r.client, plan.Type.ValueString(), plan.Filter.ValueString(), plan.Author.ValueString(), plan.Comment.ValueString(), plan.StartTime.ValueInt64(), plan.EndTime.ValueInt64(), plan.Fixed.ValueBool(), plan.Duration.ValueInt64(), plan.AllServices.ValueBool(), plan.TriggerName.ValueString(), plan.ChildOptions.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating downtime",
			fmt.Sprintf("%v", err),
		)
	}

	namesAttr := make([]attr.Value, 0, len(names))
	for _, name := range names {
		namesAttr = append(namesAttr, types.StringValue(name))
	}
	plan.Names, _ = types.ListValue(types.StringType, namesAttr)

	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *downtimeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state downtimeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *downtimeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan downtimeResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO

	// Set refreshed plan
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *downtimeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state downtimeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	downtimes := make([]types.String, 0, len(state.Names.Elements()))
	state.Names.ElementsAs(ctx, &downtimes, false)

	for _, downtime := range downtimes {
		err := DeleteDowntime(r.client, downtime.ValueString(), state.Author.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error deleting downtime",
				fmt.Sprintf("%v", err),
			)
			return
		}
	}
}

type DowntimeCreateRequest struct {
	Type         string `json:"type"`
	Filter       string `json:"filter"`
	Author       string `json:"author"`
	Comment      string `json:"comment"`
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	Fixed        bool   `json:"fixed"`
	Duration     int64  `json:"duration,omitempty"`
	AllServices  bool   `json:"all_services"`
	TriggerName  string `json:"trigger_name,omitempty"`
	ChildOptions string `json:"child_options,omitempty"`
}

type DowntimeCreateResponse struct {
	Code     float64 `json:"code"`
	LegacyID float64 `json:"legacy_id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
}

func CreateDowntime(client *iapi.Server, t string, filter string, author string, comment string, startTime int64, endTime int64, fixed bool, duration int64, allServices bool, triggerName string, childOptions string) ([]string, error) {
	payload := DowntimeCreateRequest{
		Type:         t,
		Filter:       filter,
		Author:       author,
		Comment:      comment,
		StartTime:    startTime,
		EndTime:      endTime,
		Fixed:        fixed,
		Duration:     duration,
		AllServices:  allServices,
		TriggerName:  triggerName,
		ChildOptions: childOptions,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal downtime payload: %v", err)
	}

	r, err := client.NewAPIRequest("POST", "/actions/schedule-downtime", payloadJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to POST on the API: %v", err)
	}

	if r.Code != http.StatusOK {
		return nil, fmt.Errorf("%d, got %d: %v", http.StatusOK, r.Code, r)
	}

	jsonResponse, err := json.Marshal(r.Results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the downtime response: %v", err)
	}

	var results []DowntimeCreateResponse
	err = json.Unmarshal(jsonResponse, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall the downtime response: %v", err)
	}

	var names []string
	for _, downtime := range results {
		names = append(names, downtime.Name)
	}
	return names, nil
}

type DowntimeDeleteRequest struct {
	Downtime string `json:"downtime"`
	Author   string `json:"author"`
}

type DowntimeDeleteResponse struct {
	Code   float64 `json:"code"`
	Status string  `json:"status"`
}

func DeleteDowntime(client *iapi.Server, downtime string, author string) error {
	payload := DowntimeDeleteRequest{
		Downtime: downtime,
		Author:   author,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal downtime payload: %v", err)
	}

	response, err := client.NewAPIRequest("POST", "/actions/remove-downtime", payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to POST on the API: %v", err)
	}

	if !slices.Contains([]int{http.StatusOK, http.StatusNotFound}, response.Code) {
		return fmt.Errorf("expected code %d or %d, got %d: %v", http.StatusOK, http.StatusNotFound, response.Code, response)
	}

	jsonResponse, err := json.Marshal(response.Results)
	if err != nil {
		return fmt.Errorf("failed to marshal the downtime response: %v", err)
	}

	var results []DowntimeDeleteResponse
	err = json.Unmarshal(jsonResponse, &results)
	if err != nil {
		return fmt.Errorf("failed to unmarshall the downtime response: %v", err)
	}

	for _, result := range results {
		if int(result.Code) != http.StatusOK {
			return fmt.Errorf("failed to delete downtime: %s", result.Status)
		}
	}
	return nil
}
