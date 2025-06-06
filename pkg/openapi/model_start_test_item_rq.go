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

// checks if the StartTestItemRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &StartTestItemRQ{}

// StartTestItemRQ struct for StartTestItemRQ
type StartTestItemRQ struct {
	StartTime time.Time `json:"startTime"`
	// UUID of parent launch
	LaunchUuid  string              `json:"launchUuid"`
	Name        string              `json:"name"`
	Description *string             `json:"description,omitempty"`
	Attributes  []ItemAttributesRQ  `json:"attributes,omitempty"`
	Uuid        string              `json:"uuid"`
	CodeRef     *string             `json:"codeRef,omitempty"`
	Parameters  []ParameterResource `json:"parameters,omitempty"`
	UniqueId    *string             `json:"uniqueId,omitempty"`
	TestCaseId  *string             `json:"testCaseId,omitempty"`
	Type        string              `json:"type"`
	Retry       *bool               `json:"retry,omitempty"`
	HasStats    *bool               `json:"hasStats,omitempty"`
	RetryOf     *string             `json:"retryOf,omitempty"`
}

type _StartTestItemRQ StartTestItemRQ

// NewStartTestItemRQ instantiates a new StartTestItemRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewStartTestItemRQ(startTime time.Time, launchUuid string, name string, uuid string, type_ string) *StartTestItemRQ {
	this := StartTestItemRQ{}
	this.StartTime = startTime
	this.LaunchUuid = launchUuid
	this.Name = name
	this.Uuid = uuid
	this.Type = type_
	return &this
}

// NewStartTestItemRQWithDefaults instantiates a new StartTestItemRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewStartTestItemRQWithDefaults() *StartTestItemRQ {
	this := StartTestItemRQ{}
	return &this
}

// GetStartTime returns the StartTime field value
func (o *StartTestItemRQ) GetStartTime() time.Time {
	if o == nil {
		var ret time.Time
		return ret
	}

	return o.StartTime
}

// GetStartTimeOk returns a tuple with the StartTime field value
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetStartTimeOk() (*time.Time, bool) {
	if o == nil {
		return nil, false
	}
	return &o.StartTime, true
}

// SetStartTime sets field value
func (o *StartTestItemRQ) SetStartTime(v time.Time) {
	o.StartTime = v
}

// GetLaunchUuid returns the LaunchUuid field value
func (o *StartTestItemRQ) GetLaunchUuid() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.LaunchUuid
}

// GetLaunchUuidOk returns a tuple with the LaunchUuid field value
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetLaunchUuidOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.LaunchUuid, true
}

// SetLaunchUuid sets field value
func (o *StartTestItemRQ) SetLaunchUuid(v string) {
	o.LaunchUuid = v
}

// GetName returns the Name field value
func (o *StartTestItemRQ) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *StartTestItemRQ) SetName(v string) {
	o.Name = v
}

// GetDescription returns the Description field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetDescription() string {
	if o == nil || IsNil(o.Description) {
		var ret string
		return ret
	}
	return *o.Description
}

// GetDescriptionOk returns a tuple with the Description field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetDescriptionOk() (*string, bool) {
	if o == nil || IsNil(o.Description) {
		return nil, false
	}
	return o.Description, true
}

// HasDescription returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasDescription() bool {
	if o != nil && !IsNil(o.Description) {
		return true
	}

	return false
}

// SetDescription gets a reference to the given string and assigns it to the Description field.
func (o *StartTestItemRQ) SetDescription(v string) {
	o.Description = &v
}

// GetAttributes returns the Attributes field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetAttributes() []ItemAttributesRQ {
	if o == nil || IsNil(o.Attributes) {
		var ret []ItemAttributesRQ
		return ret
	}
	return o.Attributes
}

// GetAttributesOk returns a tuple with the Attributes field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetAttributesOk() ([]ItemAttributesRQ, bool) {
	if o == nil || IsNil(o.Attributes) {
		return nil, false
	}
	return o.Attributes, true
}

// HasAttributes returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasAttributes() bool {
	if o != nil && !IsNil(o.Attributes) {
		return true
	}

	return false
}

// SetAttributes gets a reference to the given []ItemAttributesRQ and assigns it to the Attributes field.
func (o *StartTestItemRQ) SetAttributes(v []ItemAttributesRQ) {
	o.Attributes = v
}

// GetUuid returns the Uuid field value
func (o *StartTestItemRQ) GetUuid() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Uuid
}

// GetUuidOk returns a tuple with the Uuid field value
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetUuidOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Uuid, true
}

// SetUuid sets field value
func (o *StartTestItemRQ) SetUuid(v string) {
	o.Uuid = v
}

// GetCodeRef returns the CodeRef field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetCodeRef() string {
	if o == nil || IsNil(o.CodeRef) {
		var ret string
		return ret
	}
	return *o.CodeRef
}

// GetCodeRefOk returns a tuple with the CodeRef field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetCodeRefOk() (*string, bool) {
	if o == nil || IsNil(o.CodeRef) {
		return nil, false
	}
	return o.CodeRef, true
}

// HasCodeRef returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasCodeRef() bool {
	if o != nil && !IsNil(o.CodeRef) {
		return true
	}

	return false
}

// SetCodeRef gets a reference to the given string and assigns it to the CodeRef field.
func (o *StartTestItemRQ) SetCodeRef(v string) {
	o.CodeRef = &v
}

// GetParameters returns the Parameters field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetParameters() []ParameterResource {
	if o == nil || IsNil(o.Parameters) {
		var ret []ParameterResource
		return ret
	}
	return o.Parameters
}

// GetParametersOk returns a tuple with the Parameters field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetParametersOk() ([]ParameterResource, bool) {
	if o == nil || IsNil(o.Parameters) {
		return nil, false
	}
	return o.Parameters, true
}

// HasParameters returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasParameters() bool {
	if o != nil && !IsNil(o.Parameters) {
		return true
	}

	return false
}

// SetParameters gets a reference to the given []ParameterResource and assigns it to the Parameters field.
func (o *StartTestItemRQ) SetParameters(v []ParameterResource) {
	o.Parameters = v
}

// GetUniqueId returns the UniqueId field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetUniqueId() string {
	if o == nil || IsNil(o.UniqueId) {
		var ret string
		return ret
	}
	return *o.UniqueId
}

// GetUniqueIdOk returns a tuple with the UniqueId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetUniqueIdOk() (*string, bool) {
	if o == nil || IsNil(o.UniqueId) {
		return nil, false
	}
	return o.UniqueId, true
}

// HasUniqueId returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasUniqueId() bool {
	if o != nil && !IsNil(o.UniqueId) {
		return true
	}

	return false
}

// SetUniqueId gets a reference to the given string and assigns it to the UniqueId field.
func (o *StartTestItemRQ) SetUniqueId(v string) {
	o.UniqueId = &v
}

// GetTestCaseId returns the TestCaseId field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetTestCaseId() string {
	if o == nil || IsNil(o.TestCaseId) {
		var ret string
		return ret
	}
	return *o.TestCaseId
}

// GetTestCaseIdOk returns a tuple with the TestCaseId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetTestCaseIdOk() (*string, bool) {
	if o == nil || IsNil(o.TestCaseId) {
		return nil, false
	}
	return o.TestCaseId, true
}

// HasTestCaseId returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasTestCaseId() bool {
	if o != nil && !IsNil(o.TestCaseId) {
		return true
	}

	return false
}

// SetTestCaseId gets a reference to the given string and assigns it to the TestCaseId field.
func (o *StartTestItemRQ) SetTestCaseId(v string) {
	o.TestCaseId = &v
}

// GetType returns the Type field value
func (o *StartTestItemRQ) GetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Type
}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Type, true
}

// SetType sets field value
func (o *StartTestItemRQ) SetType(v string) {
	o.Type = v
}

// GetRetry returns the Retry field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetRetry() bool {
	if o == nil || IsNil(o.Retry) {
		var ret bool
		return ret
	}
	return *o.Retry
}

// GetRetryOk returns a tuple with the Retry field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetRetryOk() (*bool, bool) {
	if o == nil || IsNil(o.Retry) {
		return nil, false
	}
	return o.Retry, true
}

// HasRetry returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasRetry() bool {
	if o != nil && !IsNil(o.Retry) {
		return true
	}

	return false
}

// SetRetry gets a reference to the given bool and assigns it to the Retry field.
func (o *StartTestItemRQ) SetRetry(v bool) {
	o.Retry = &v
}

// GetHasStats returns the HasStats field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetHasStats() bool {
	if o == nil || IsNil(o.HasStats) {
		var ret bool
		return ret
	}
	return *o.HasStats
}

// GetHasStatsOk returns a tuple with the HasStats field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetHasStatsOk() (*bool, bool) {
	if o == nil || IsNil(o.HasStats) {
		return nil, false
	}
	return o.HasStats, true
}

// HasHasStats returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasHasStats() bool {
	if o != nil && !IsNil(o.HasStats) {
		return true
	}

	return false
}

// SetHasStats gets a reference to the given bool and assigns it to the HasStats field.
func (o *StartTestItemRQ) SetHasStats(v bool) {
	o.HasStats = &v
}

// GetRetryOf returns the RetryOf field value if set, zero value otherwise.
func (o *StartTestItemRQ) GetRetryOf() string {
	if o == nil || IsNil(o.RetryOf) {
		var ret string
		return ret
	}
	return *o.RetryOf
}

// GetRetryOfOk returns a tuple with the RetryOf field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *StartTestItemRQ) GetRetryOfOk() (*string, bool) {
	if o == nil || IsNil(o.RetryOf) {
		return nil, false
	}
	return o.RetryOf, true
}

// HasRetryOf returns a boolean if a field has been set.
func (o *StartTestItemRQ) HasRetryOf() bool {
	if o != nil && !IsNil(o.RetryOf) {
		return true
	}

	return false
}

// SetRetryOf gets a reference to the given string and assigns it to the RetryOf field.
func (o *StartTestItemRQ) SetRetryOf(v string) {
	o.RetryOf = &v
}

func (o StartTestItemRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o StartTestItemRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["startTime"] = o.StartTime
	toSerialize["launchUuid"] = o.LaunchUuid
	toSerialize["name"] = o.Name
	if !IsNil(o.Description) {
		toSerialize["description"] = o.Description
	}
	if !IsNil(o.Attributes) {
		toSerialize["attributes"] = o.Attributes
	}
	toSerialize["uuid"] = o.Uuid
	if !IsNil(o.CodeRef) {
		toSerialize["codeRef"] = o.CodeRef
	}
	if !IsNil(o.Parameters) {
		toSerialize["parameters"] = o.Parameters
	}
	if !IsNil(o.UniqueId) {
		toSerialize["uniqueId"] = o.UniqueId
	}
	if !IsNil(o.TestCaseId) {
		toSerialize["testCaseId"] = o.TestCaseId
	}
	toSerialize["type"] = o.Type
	if !IsNil(o.Retry) {
		toSerialize["retry"] = o.Retry
	}
	if !IsNil(o.HasStats) {
		toSerialize["hasStats"] = o.HasStats
	}
	if !IsNil(o.RetryOf) {
		toSerialize["retryOf"] = o.RetryOf
	}
	return toSerialize, nil
}

func (o *StartTestItemRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"startTime",
		"launchUuid",
		"name",
		"uuid",
		"type",
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

	varStartTestItemRQ := _StartTestItemRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varStartTestItemRQ)

	if err != nil {
		return err
	}

	*o = StartTestItemRQ(varStartTestItemRQ)

	return err
}

type NullableStartTestItemRQ struct {
	value *StartTestItemRQ
	isSet bool
}

func (v NullableStartTestItemRQ) Get() *StartTestItemRQ {
	return v.value
}

func (v *NullableStartTestItemRQ) Set(val *StartTestItemRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableStartTestItemRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableStartTestItemRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableStartTestItemRQ(val *StartTestItemRQ) *NullableStartTestItemRQ {
	return &NullableStartTestItemRQ{value: val, isSet: true}
}

func (v NullableStartTestItemRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableStartTestItemRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
