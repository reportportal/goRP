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

// checks if the DashboardPreconfiguredRq type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &DashboardPreconfiguredRq{}

// DashboardPreconfiguredRq struct for DashboardPreconfiguredRq
type DashboardPreconfiguredRq struct {
	Description *string                 `json:"description,omitempty"`
	Name        string                  `json:"name"`
	Config      DashboardConfigResource `json:"config"`
}

type _DashboardPreconfiguredRq DashboardPreconfiguredRq

// NewDashboardPreconfiguredRq instantiates a new DashboardPreconfiguredRq object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewDashboardPreconfiguredRq(name string, config DashboardConfigResource) *DashboardPreconfiguredRq {
	this := DashboardPreconfiguredRq{}
	this.Name = name
	this.Config = config
	return &this
}

// NewDashboardPreconfiguredRqWithDefaults instantiates a new DashboardPreconfiguredRq object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewDashboardPreconfiguredRqWithDefaults() *DashboardPreconfiguredRq {
	this := DashboardPreconfiguredRq{}
	return &this
}

// GetDescription returns the Description field value if set, zero value otherwise.
func (o *DashboardPreconfiguredRq) GetDescription() string {
	if o == nil || IsNil(o.Description) {
		var ret string
		return ret
	}
	return *o.Description
}

// GetDescriptionOk returns a tuple with the Description field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *DashboardPreconfiguredRq) GetDescriptionOk() (*string, bool) {
	if o == nil || IsNil(o.Description) {
		return nil, false
	}
	return o.Description, true
}

// HasDescription returns a boolean if a field has been set.
func (o *DashboardPreconfiguredRq) HasDescription() bool {
	if o != nil && !IsNil(o.Description) {
		return true
	}

	return false
}

// SetDescription gets a reference to the given string and assigns it to the Description field.
func (o *DashboardPreconfiguredRq) SetDescription(v string) {
	o.Description = &v
}

// GetName returns the Name field value
func (o *DashboardPreconfiguredRq) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *DashboardPreconfiguredRq) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *DashboardPreconfiguredRq) SetName(v string) {
	o.Name = v
}

// GetConfig returns the Config field value
func (o *DashboardPreconfiguredRq) GetConfig() DashboardConfigResource {
	if o == nil {
		var ret DashboardConfigResource
		return ret
	}

	return o.Config
}

// GetConfigOk returns a tuple with the Config field value
// and a boolean to check if the value has been set.
func (o *DashboardPreconfiguredRq) GetConfigOk() (*DashboardConfigResource, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Config, true
}

// SetConfig sets field value
func (o *DashboardPreconfiguredRq) SetConfig(v DashboardConfigResource) {
	o.Config = v
}

func (o DashboardPreconfiguredRq) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o DashboardPreconfiguredRq) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Description) {
		toSerialize["description"] = o.Description
	}
	toSerialize["name"] = o.Name
	toSerialize["config"] = o.Config
	return toSerialize, nil
}

func (o *DashboardPreconfiguredRq) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"name",
		"config",
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

	varDashboardPreconfiguredRq := _DashboardPreconfiguredRq{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varDashboardPreconfiguredRq)

	if err != nil {
		return err
	}

	*o = DashboardPreconfiguredRq(varDashboardPreconfiguredRq)

	return err
}

type NullableDashboardPreconfiguredRq struct {
	value *DashboardPreconfiguredRq
	isSet bool
}

func (v NullableDashboardPreconfiguredRq) Get() *DashboardPreconfiguredRq {
	return v.value
}

func (v *NullableDashboardPreconfiguredRq) Set(val *DashboardPreconfiguredRq) {
	v.value = val
	v.isSet = true
}

func (v NullableDashboardPreconfiguredRq) IsSet() bool {
	return v.isSet
}

func (v *NullableDashboardPreconfiguredRq) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableDashboardPreconfiguredRq(val *DashboardPreconfiguredRq) *NullableDashboardPreconfiguredRq {
	return &NullableDashboardPreconfiguredRq{value: val, isSet: true}
}

func (v NullableDashboardPreconfiguredRq) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableDashboardPreconfiguredRq) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
