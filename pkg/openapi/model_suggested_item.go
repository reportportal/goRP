/*
ReportPortal

ReportPortal API documentation

API version: develop-322
Contact: support@reportportal.io
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"encoding/json"
)

// checks if the SuggestedItem type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &SuggestedItem{}

// SuggestedItem struct for SuggestedItem
type SuggestedItem struct {
	TestItemResource *TestItemResource `json:"testItemResource,omitempty"`
	Logs             []LogResource     `json:"logs,omitempty"`
	SuggestRs        *SuggestInfo      `json:"suggestRs,omitempty"`
}

// NewSuggestedItem instantiates a new SuggestedItem object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewSuggestedItem() *SuggestedItem {
	this := SuggestedItem{}
	return &this
}

// NewSuggestedItemWithDefaults instantiates a new SuggestedItem object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewSuggestedItemWithDefaults() *SuggestedItem {
	this := SuggestedItem{}
	return &this
}

// GetTestItemResource returns the TestItemResource field value if set, zero value otherwise.
func (o *SuggestedItem) GetTestItemResource() TestItemResource {
	if o == nil || IsNil(o.TestItemResource) {
		var ret TestItemResource
		return ret
	}
	return *o.TestItemResource
}

// GetTestItemResourceOk returns a tuple with the TestItemResource field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SuggestedItem) GetTestItemResourceOk() (*TestItemResource, bool) {
	if o == nil || IsNil(o.TestItemResource) {
		return nil, false
	}
	return o.TestItemResource, true
}

// HasTestItemResource returns a boolean if a field has been set.
func (o *SuggestedItem) HasTestItemResource() bool {
	if o != nil && !IsNil(o.TestItemResource) {
		return true
	}

	return false
}

// SetTestItemResource gets a reference to the given TestItemResource and assigns it to the TestItemResource field.
func (o *SuggestedItem) SetTestItemResource(v TestItemResource) {
	o.TestItemResource = &v
}

// GetLogs returns the Logs field value if set, zero value otherwise.
func (o *SuggestedItem) GetLogs() []LogResource {
	if o == nil || IsNil(o.Logs) {
		var ret []LogResource
		return ret
	}
	return o.Logs
}

// GetLogsOk returns a tuple with the Logs field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SuggestedItem) GetLogsOk() ([]LogResource, bool) {
	if o == nil || IsNil(o.Logs) {
		return nil, false
	}
	return o.Logs, true
}

// HasLogs returns a boolean if a field has been set.
func (o *SuggestedItem) HasLogs() bool {
	if o != nil && !IsNil(o.Logs) {
		return true
	}

	return false
}

// SetLogs gets a reference to the given []LogResource and assigns it to the Logs field.
func (o *SuggestedItem) SetLogs(v []LogResource) {
	o.Logs = v
}

// GetSuggestRs returns the SuggestRs field value if set, zero value otherwise.
func (o *SuggestedItem) GetSuggestRs() SuggestInfo {
	if o == nil || IsNil(o.SuggestRs) {
		var ret SuggestInfo
		return ret
	}
	return *o.SuggestRs
}

// GetSuggestRsOk returns a tuple with the SuggestRs field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SuggestedItem) GetSuggestRsOk() (*SuggestInfo, bool) {
	if o == nil || IsNil(o.SuggestRs) {
		return nil, false
	}
	return o.SuggestRs, true
}

// HasSuggestRs returns a boolean if a field has been set.
func (o *SuggestedItem) HasSuggestRs() bool {
	if o != nil && !IsNil(o.SuggestRs) {
		return true
	}

	return false
}

// SetSuggestRs gets a reference to the given SuggestInfo and assigns it to the SuggestRs field.
func (o *SuggestedItem) SetSuggestRs(v SuggestInfo) {
	o.SuggestRs = &v
}

func (o SuggestedItem) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o SuggestedItem) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.TestItemResource) {
		toSerialize["testItemResource"] = o.TestItemResource
	}
	if !IsNil(o.Logs) {
		toSerialize["logs"] = o.Logs
	}
	if !IsNil(o.SuggestRs) {
		toSerialize["suggestRs"] = o.SuggestRs
	}
	return toSerialize, nil
}

type NullableSuggestedItem struct {
	value *SuggestedItem
	isSet bool
}

func (v NullableSuggestedItem) Get() *SuggestedItem {
	return v.value
}

func (v *NullableSuggestedItem) Set(val *SuggestedItem) {
	v.value = val
	v.isSet = true
}

func (v NullableSuggestedItem) IsSet() bool {
	return v.isSet
}

func (v *NullableSuggestedItem) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableSuggestedItem(val *SuggestedItem) *NullableSuggestedItem {
	return &NullableSuggestedItem{value: val, isSet: true}
}

func (v NullableSuggestedItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableSuggestedItem) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
