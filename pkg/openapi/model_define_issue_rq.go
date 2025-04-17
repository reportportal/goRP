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
)

// checks if the DefineIssueRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &DefineIssueRQ{}

// DefineIssueRQ struct for DefineIssueRQ
type DefineIssueRQ struct {
	Issues []IssueDefinition `json:"issues"`
}

type _DefineIssueRQ DefineIssueRQ

// NewDefineIssueRQ instantiates a new DefineIssueRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewDefineIssueRQ(issues []IssueDefinition) *DefineIssueRQ {
	this := DefineIssueRQ{}
	this.Issues = issues
	return &this
}

// NewDefineIssueRQWithDefaults instantiates a new DefineIssueRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewDefineIssueRQWithDefaults() *DefineIssueRQ {
	this := DefineIssueRQ{}
	return &this
}

// GetIssues returns the Issues field value
func (o *DefineIssueRQ) GetIssues() []IssueDefinition {
	if o == nil {
		var ret []IssueDefinition
		return ret
	}

	return o.Issues
}

// GetIssuesOk returns a tuple with the Issues field value
// and a boolean to check if the value has been set.
func (o *DefineIssueRQ) GetIssuesOk() ([]IssueDefinition, bool) {
	if o == nil {
		return nil, false
	}
	return o.Issues, true
}

// SetIssues sets field value
func (o *DefineIssueRQ) SetIssues(v []IssueDefinition) {
	o.Issues = v
}

func (o DefineIssueRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o DefineIssueRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["issues"] = o.Issues
	return toSerialize, nil
}

func (o *DefineIssueRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"issues",
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

	varDefineIssueRQ := _DefineIssueRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varDefineIssueRQ)

	if err != nil {
		return err
	}

	*o = DefineIssueRQ(varDefineIssueRQ)

	return err
}

type NullableDefineIssueRQ struct {
	value *DefineIssueRQ
	isSet bool
}

func (v NullableDefineIssueRQ) Get() *DefineIssueRQ {
	return v.value
}

func (v *NullableDefineIssueRQ) Set(val *DefineIssueRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableDefineIssueRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableDefineIssueRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableDefineIssueRQ(val *DefineIssueRQ) *NullableDefineIssueRQ {
	return &NullableDefineIssueRQ{value: val, isSet: true}
}

func (v NullableDefineIssueRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableDefineIssueRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
