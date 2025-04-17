/*
ReportPortal

ReportPortal API documentation

API version: develop-322
Contact: support@reportportal.io
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// checks if the LaunchResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &LaunchResource{}

// LaunchResource struct for LaunchResource
type LaunchResource struct {
	Owner               *string                 `json:"owner,omitempty"`
	Description         *string                 `json:"description,omitempty"`
	Id                  int64                   `json:"id"`
	Uuid                string                  `json:"uuid"`
	Name                string                  `json:"name"`
	Number              int64                   `json:"number"`
	StartTime           time.Time               `json:"startTime"`
	EndTime             *time.Time              `json:"endTime,omitempty"`
	LastModified        *time.Time              `json:"lastModified,omitempty"`
	Status              string                  `json:"status"`
	Statistics          *StatisticsResource     `json:"statistics,omitempty"`
	Attributes          []ItemAttributeResource `json:"attributes,omitempty"`
	Mode                *string                 `json:"mode,omitempty"`
	Analysing           []string                `json:"analysing,omitempty"`
	ApproximateDuration *float64                `json:"approximateDuration,omitempty"`
	HasRetries          *bool                   `json:"hasRetries,omitempty"`
	Rerun               *bool                   `json:"rerun,omitempty"`
	Metadata            map[string]interface{}  `json:"metadata,omitempty"`
	RetentionPolicy     *string                 `json:"retentionPolicy,omitempty"`
}

type _LaunchResource LaunchResource

// NewLaunchResource instantiates a new LaunchResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewLaunchResource(id int64, uuid, name string, number int64, startTime time.Time, status string) *LaunchResource {
	this := LaunchResource{}
	this.Id = id
	this.Uuid = uuid
	this.Name = name
	this.Number = number
	this.StartTime = startTime
	this.Status = status
	return &this
}

// NewLaunchResourceWithDefaults instantiates a new LaunchResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewLaunchResourceWithDefaults() *LaunchResource {
	this := LaunchResource{}
	return &this
}

// GetOwner returns the Owner field value if set, zero value otherwise.
func (o *LaunchResource) GetOwner() string {
	if o == nil || IsNil(o.Owner) {
		var ret string
		return ret
	}
	return *o.Owner
}

// GetOwnerOk returns a tuple with the Owner field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetOwnerOk() (*string, bool) {
	if o == nil || IsNil(o.Owner) {
		return nil, false
	}
	return o.Owner, true
}

// HasOwner returns a boolean if a field has been set.
func (o *LaunchResource) HasOwner() bool {
	if o != nil && !IsNil(o.Owner) {
		return true
	}

	return false
}

// SetOwner gets a reference to the given string and assigns it to the Owner field.
func (o *LaunchResource) SetOwner(v string) {
	o.Owner = &v
}

// GetDescription returns the Description field value if set, zero value otherwise.
func (o *LaunchResource) GetDescription() string {
	if o == nil || IsNil(o.Description) {
		var ret string
		return ret
	}
	return *o.Description
}

// GetDescriptionOk returns a tuple with the Description field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetDescriptionOk() (*string, bool) {
	if o == nil || IsNil(o.Description) {
		return nil, false
	}
	return o.Description, true
}

// HasDescription returns a boolean if a field has been set.
func (o *LaunchResource) HasDescription() bool {
	if o != nil && !IsNil(o.Description) {
		return true
	}

	return false
}

// SetDescription gets a reference to the given string and assigns it to the Description field.
func (o *LaunchResource) SetDescription(v string) {
	o.Description = &v
}

// GetId returns the Id field value
func (o *LaunchResource) GetId() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetIdOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *LaunchResource) SetId(v int64) {
	o.Id = v
}

// GetUuid returns the Uuid field value
func (o *LaunchResource) GetUuid() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Uuid
}

// GetUuidOk returns a tuple with the Uuid field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetUuidOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Uuid, true
}

// SetUuid sets field value
func (o *LaunchResource) SetUuid(v string) {
	o.Uuid = v
}

// GetName returns the Name field value
func (o *LaunchResource) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *LaunchResource) SetName(v string) {
	o.Name = v
}

// GetNumber returns the Number field value
func (o *LaunchResource) GetNumber() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Number
}

// GetNumberOk returns a tuple with the Number field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetNumberOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Number, true
}

// SetNumber sets field value
func (o *LaunchResource) SetNumber(v int64) {
	o.Number = v
}

// GetStartTime returns the StartTime field value
func (o *LaunchResource) GetStartTime() time.Time {
	if o == nil {
		var ret time.Time
		return ret
	}

	return o.StartTime
}

// GetStartTimeOk returns a tuple with the StartTime field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetStartTimeOk() (*time.Time, bool) {
	if o == nil {
		return nil, false
	}
	return &o.StartTime, true
}

// SetStartTime sets field value
func (o *LaunchResource) SetStartTime(v time.Time) {
	o.StartTime = v
}

// GetEndTime returns the EndTime field value if set, zero value otherwise.
func (o *LaunchResource) GetEndTime() time.Time {
	if o == nil || IsNil(o.EndTime) {
		var ret time.Time
		return ret
	}
	return *o.EndTime
}

// GetEndTimeOk returns a tuple with the EndTime field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetEndTimeOk() (*time.Time, bool) {
	if o == nil || IsNil(o.EndTime) {
		return nil, false
	}
	return o.EndTime, true
}

// HasEndTime returns a boolean if a field has been set.
func (o *LaunchResource) HasEndTime() bool {
	if o != nil && !IsNil(o.EndTime) {
		return true
	}

	return false
}

// SetEndTime gets a reference to the given time.Time and assigns it to the EndTime field.
func (o *LaunchResource) SetEndTime(v time.Time) {
	o.EndTime = &v
}

// GetLastModified returns the LastModified field value if set, zero value otherwise.
func (o *LaunchResource) GetLastModified() time.Time {
	if o == nil || IsNil(o.LastModified) {
		var ret time.Time
		return ret
	}
	return *o.LastModified
}

// GetLastModifiedOk returns a tuple with the LastModified field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetLastModifiedOk() (*time.Time, bool) {
	if o == nil || IsNil(o.LastModified) {
		return nil, false
	}
	return o.LastModified, true
}

// HasLastModified returns a boolean if a field has been set.
func (o *LaunchResource) HasLastModified() bool {
	if o != nil && !IsNil(o.LastModified) {
		return true
	}

	return false
}

// SetLastModified gets a reference to the given time.Time and assigns it to the LastModified field.
func (o *LaunchResource) SetLastModified(v time.Time) {
	o.LastModified = &v
}

// GetStatus returns the Status field value
func (o *LaunchResource) GetStatus() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Status
}

// GetStatusOk returns a tuple with the Status field value
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetStatusOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Status, true
}

// SetStatus sets field value
func (o *LaunchResource) SetStatus(v string) {
	o.Status = v
}

// GetStatistics returns the Statistics field value if set, zero value otherwise.
func (o *LaunchResource) GetStatistics() StatisticsResource {
	if o == nil || IsNil(o.Statistics) {
		var ret StatisticsResource
		return ret
	}
	return *o.Statistics
}

// GetStatisticsOk returns a tuple with the Statistics field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetStatisticsOk() (*StatisticsResource, bool) {
	if o == nil || IsNil(o.Statistics) {
		return nil, false
	}
	return o.Statistics, true
}

// HasStatistics returns a boolean if a field has been set.
func (o *LaunchResource) HasStatistics() bool {
	if o != nil && !IsNil(o.Statistics) {
		return true
	}

	return false
}

// SetStatistics gets a reference to the given StatisticsResource and assigns it to the Statistics field.
func (o *LaunchResource) SetStatistics(v StatisticsResource) {
	o.Statistics = &v
}

// GetAttributes returns the Attributes field value if set, zero value otherwise.
func (o *LaunchResource) GetAttributes() []ItemAttributeResource {
	if o == nil || IsNil(o.Attributes) {
		var ret []ItemAttributeResource
		return ret
	}
	return o.Attributes
}

// GetAttributesOk returns a tuple with the Attributes field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetAttributesOk() ([]ItemAttributeResource, bool) {
	if o == nil || IsNil(o.Attributes) {
		return nil, false
	}
	return o.Attributes, true
}

// HasAttributes returns a boolean if a field has been set.
func (o *LaunchResource) HasAttributes() bool {
	if o != nil && !IsNil(o.Attributes) {
		return true
	}

	return false
}

// SetAttributes gets a reference to the given []ItemAttributeResource and assigns it to the Attributes field.
func (o *LaunchResource) SetAttributes(v []ItemAttributeResource) {
	o.Attributes = v
}

// GetMode returns the Mode field value if set, zero value otherwise.
func (o *LaunchResource) GetMode() string {
	if o == nil || IsNil(o.Mode) {
		var ret string
		return ret
	}
	return *o.Mode
}

// GetModeOk returns a tuple with the Mode field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetModeOk() (*string, bool) {
	if o == nil || IsNil(o.Mode) {
		return nil, false
	}
	return o.Mode, true
}

// HasMode returns a boolean if a field has been set.
func (o *LaunchResource) HasMode() bool {
	if o != nil && !IsNil(o.Mode) {
		return true
	}

	return false
}

// SetMode gets a reference to the given string and assigns it to the Mode field.
func (o *LaunchResource) SetMode(v string) {
	o.Mode = &v
}

// GetAnalysing returns the Analysing field value if set, zero value otherwise.
func (o *LaunchResource) GetAnalysing() []string {
	if o == nil || IsNil(o.Analysing) {
		var ret []string
		return ret
	}
	return o.Analysing
}

// GetAnalysingOk returns a tuple with the Analysing field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetAnalysingOk() ([]string, bool) {
	if o == nil || IsNil(o.Analysing) {
		return nil, false
	}
	return o.Analysing, true
}

// HasAnalysing returns a boolean if a field has been set.
func (o *LaunchResource) HasAnalysing() bool {
	if o != nil && !IsNil(o.Analysing) {
		return true
	}

	return false
}

// SetAnalysing gets a reference to the given []string and assigns it to the Analysing field.
func (o *LaunchResource) SetAnalysing(v []string) {
	o.Analysing = v
}

// GetApproximateDuration returns the ApproximateDuration field value if set, zero value otherwise.
func (o *LaunchResource) GetApproximateDuration() float64 {
	if o == nil || IsNil(o.ApproximateDuration) {
		var ret float64
		return ret
	}
	return *o.ApproximateDuration
}

// GetApproximateDurationOk returns a tuple with the ApproximateDuration field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetApproximateDurationOk() (*float64, bool) {
	if o == nil || IsNil(o.ApproximateDuration) {
		return nil, false
	}
	return o.ApproximateDuration, true
}

// HasApproximateDuration returns a boolean if a field has been set.
func (o *LaunchResource) HasApproximateDuration() bool {
	if o != nil && !IsNil(o.ApproximateDuration) {
		return true
	}

	return false
}

// SetApproximateDuration gets a reference to the given float64 and assigns it to the ApproximateDuration field.
func (o *LaunchResource) SetApproximateDuration(v float64) {
	o.ApproximateDuration = &v
}

// GetHasRetries returns the HasRetries field value if set, zero value otherwise.
func (o *LaunchResource) GetHasRetries() bool {
	if o == nil || IsNil(o.HasRetries) {
		var ret bool
		return ret
	}
	return *o.HasRetries
}

// GetHasRetriesOk returns a tuple with the HasRetries field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetHasRetriesOk() (*bool, bool) {
	if o == nil || IsNil(o.HasRetries) {
		return nil, false
	}
	return o.HasRetries, true
}

// HasHasRetries returns a boolean if a field has been set.
func (o *LaunchResource) HasHasRetries() bool {
	if o != nil && !IsNil(o.HasRetries) {
		return true
	}

	return false
}

// SetHasRetries gets a reference to the given bool and assigns it to the HasRetries field.
func (o *LaunchResource) SetHasRetries(v bool) {
	o.HasRetries = &v
}

// GetRerun returns the Rerun field value if set, zero value otherwise.
func (o *LaunchResource) GetRerun() bool {
	if o == nil || IsNil(o.Rerun) {
		var ret bool
		return ret
	}
	return *o.Rerun
}

// GetRerunOk returns a tuple with the Rerun field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetRerunOk() (*bool, bool) {
	if o == nil || IsNil(o.Rerun) {
		return nil, false
	}
	return o.Rerun, true
}

// HasRerun returns a boolean if a field has been set.
func (o *LaunchResource) HasRerun() bool {
	if o != nil && !IsNil(o.Rerun) {
		return true
	}

	return false
}

// SetRerun gets a reference to the given bool and assigns it to the Rerun field.
func (o *LaunchResource) SetRerun(v bool) {
	o.Rerun = &v
}

// GetMetadata returns the Metadata field value if set, zero value otherwise.
func (o *LaunchResource) GetMetadata() map[string]interface{} {
	if o == nil || IsNil(o.Metadata) {
		var ret map[string]interface{}
		return ret
	}
	return o.Metadata
}

// GetMetadataOk returns a tuple with the Metadata field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetMetadataOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Metadata) {
		return map[string]interface{}{}, false
	}
	return o.Metadata, true
}

// HasMetadata returns a boolean if a field has been set.
func (o *LaunchResource) HasMetadata() bool {
	if o != nil && !IsNil(o.Metadata) {
		return true
	}

	return false
}

// SetMetadata gets a reference to the given map[string]interface{} and assigns it to the Metadata field.
func (o *LaunchResource) SetMetadata(v map[string]interface{}) {
	o.Metadata = v
}

// GetRetentionPolicy returns the RetentionPolicy field value if set, zero value otherwise.
func (o *LaunchResource) GetRetentionPolicy() string {
	if o == nil || IsNil(o.RetentionPolicy) {
		var ret string
		return ret
	}
	return *o.RetentionPolicy
}

// GetRetentionPolicyOk returns a tuple with the RetentionPolicy field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *LaunchResource) GetRetentionPolicyOk() (*string, bool) {
	if o == nil || IsNil(o.RetentionPolicy) {
		return nil, false
	}
	return o.RetentionPolicy, true
}

// HasRetentionPolicy returns a boolean if a field has been set.
func (o *LaunchResource) HasRetentionPolicy() bool {
	if o != nil && !IsNil(o.RetentionPolicy) {
		return true
	}

	return false
}

// SetRetentionPolicy gets a reference to the given string and assigns it to the RetentionPolicy field.
func (o *LaunchResource) SetRetentionPolicy(v string) {
	o.RetentionPolicy = &v
}

func (o LaunchResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o LaunchResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Owner) {
		toSerialize["owner"] = o.Owner
	}
	if !IsNil(o.Description) {
		toSerialize["description"] = o.Description
	}
	toSerialize["id"] = o.Id
	toSerialize["uuid"] = o.Uuid
	toSerialize["name"] = o.Name
	toSerialize["number"] = o.Number
	toSerialize["startTime"] = o.StartTime
	if !IsNil(o.EndTime) {
		toSerialize["endTime"] = o.EndTime
	}
	if !IsNil(o.LastModified) {
		toSerialize["lastModified"] = o.LastModified
	}
	toSerialize["status"] = o.Status
	if !IsNil(o.Statistics) {
		toSerialize["statistics"] = o.Statistics
	}
	if !IsNil(o.Attributes) {
		toSerialize["attributes"] = o.Attributes
	}
	if !IsNil(o.Mode) {
		toSerialize["mode"] = o.Mode
	}
	if !IsNil(o.Analysing) {
		toSerialize["analysing"] = o.Analysing
	}
	if !IsNil(o.ApproximateDuration) {
		toSerialize["approximateDuration"] = o.ApproximateDuration
	}
	if !IsNil(o.HasRetries) {
		toSerialize["hasRetries"] = o.HasRetries
	}
	if !IsNil(o.Rerun) {
		toSerialize["rerun"] = o.Rerun
	}
	if !IsNil(o.Metadata) {
		toSerialize["metadata"] = o.Metadata
	}
	if !IsNil(o.RetentionPolicy) {
		toSerialize["retentionPolicy"] = o.RetentionPolicy
	}
	return toSerialize, nil
}

func (o *LaunchResource) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"id",
		"uuid",
		"name",
		"number",
		"startTime",
		"status",
	}

	allProperties := make(map[string]interface{})

	err = json.Unmarshal(data, &allProperties)

	if err != nil {
		return err
	}

	for _, requiredProperty := range requiredProperties {
		if _, exists := allProperties[requiredProperty]; !exists {
			return fmt.Errorf("no value given for required property %v", requiredProperty)
		}
	}

	varLaunchResource := _LaunchResource{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varLaunchResource)

	if err != nil {
		return err
	}

	*o = LaunchResource(varLaunchResource)

	return err
}

type NullableLaunchResource struct {
	value *LaunchResource
	isSet bool
}

func (v NullableLaunchResource) Get() *LaunchResource {
	return v.value
}

func (v *NullableLaunchResource) Set(val *LaunchResource) {
	v.value = val
	v.isSet = true
}

func (v NullableLaunchResource) IsSet() bool {
	return v.isSet
}

func (v *NullableLaunchResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableLaunchResource(val *LaunchResource) *NullableLaunchResource {
	return &NullableLaunchResource{value: val, isSet: true}
}

func (v NullableLaunchResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableLaunchResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
