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

// checks if the UpdatePatternTemplateRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &UpdatePatternTemplateRQ{}

// UpdatePatternTemplateRQ struct for UpdatePatternTemplateRQ
type UpdatePatternTemplateRQ struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type _UpdatePatternTemplateRQ UpdatePatternTemplateRQ

// NewUpdatePatternTemplateRQ instantiates a new UpdatePatternTemplateRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewUpdatePatternTemplateRQ(name string, enabled bool) *UpdatePatternTemplateRQ {
	this := UpdatePatternTemplateRQ{}
	this.Name = name
	this.Enabled = enabled
	return &this
}

// NewUpdatePatternTemplateRQWithDefaults instantiates a new UpdatePatternTemplateRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewUpdatePatternTemplateRQWithDefaults() *UpdatePatternTemplateRQ {
	this := UpdatePatternTemplateRQ{}
	return &this
}

// GetName returns the Name field value
func (o *UpdatePatternTemplateRQ) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *UpdatePatternTemplateRQ) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *UpdatePatternTemplateRQ) SetName(v string) {
	o.Name = v
}

// GetEnabled returns the Enabled field value
func (o *UpdatePatternTemplateRQ) GetEnabled() bool {
	if o == nil {
		var ret bool
		return ret
	}

	return o.Enabled
}

// GetEnabledOk returns a tuple with the Enabled field value
// and a boolean to check if the value has been set.
func (o *UpdatePatternTemplateRQ) GetEnabledOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Enabled, true
}

// SetEnabled sets field value
func (o *UpdatePatternTemplateRQ) SetEnabled(v bool) {
	o.Enabled = v
}

func (o UpdatePatternTemplateRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o UpdatePatternTemplateRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["name"] = o.Name
	toSerialize["enabled"] = o.Enabled
	return toSerialize, nil
}

func (o *UpdatePatternTemplateRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"name",
		"enabled",
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

	varUpdatePatternTemplateRQ := _UpdatePatternTemplateRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varUpdatePatternTemplateRQ)

	if err != nil {
		return err
	}

	*o = UpdatePatternTemplateRQ(varUpdatePatternTemplateRQ)

	return err
}

type NullableUpdatePatternTemplateRQ struct {
	value *UpdatePatternTemplateRQ
	isSet bool
}

func (v NullableUpdatePatternTemplateRQ) Get() *UpdatePatternTemplateRQ {
	return v.value
}

func (v *NullableUpdatePatternTemplateRQ) Set(val *UpdatePatternTemplateRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableUpdatePatternTemplateRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableUpdatePatternTemplateRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableUpdatePatternTemplateRQ(val *UpdatePatternTemplateRQ) *NullableUpdatePatternTemplateRQ {
	return &NullableUpdatePatternTemplateRQ{value: val, isSet: true}
}

func (v NullableUpdatePatternTemplateRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableUpdatePatternTemplateRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
