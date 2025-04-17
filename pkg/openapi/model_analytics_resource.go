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

// checks if the AnalyticsResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &AnalyticsResource{}

// AnalyticsResource struct for AnalyticsResource
type AnalyticsResource struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Type    string `json:"type"`
}

type _AnalyticsResource AnalyticsResource

// NewAnalyticsResource instantiates a new AnalyticsResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewAnalyticsResource(type_ string) *AnalyticsResource {
	this := AnalyticsResource{}
	this.Type = type_
	return &this
}

// NewAnalyticsResourceWithDefaults instantiates a new AnalyticsResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewAnalyticsResourceWithDefaults() *AnalyticsResource {
	this := AnalyticsResource{}
	return &this
}

// GetEnabled returns the Enabled field value if set, zero value otherwise.
func (o *AnalyticsResource) GetEnabled() bool {
	if o == nil || IsNil(o.Enabled) {
		var ret bool
		return ret
	}
	return *o.Enabled
}

// GetEnabledOk returns a tuple with the Enabled field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AnalyticsResource) GetEnabledOk() (*bool, bool) {
	if o == nil || IsNil(o.Enabled) {
		return nil, false
	}
	return o.Enabled, true
}

// HasEnabled returns a boolean if a field has been set.
func (o *AnalyticsResource) HasEnabled() bool {
	if o != nil && !IsNil(o.Enabled) {
		return true
	}

	return false
}

// SetEnabled gets a reference to the given bool and assigns it to the Enabled field.
func (o *AnalyticsResource) SetEnabled(v bool) {
	o.Enabled = &v
}

// GetType returns the Type field value
func (o *AnalyticsResource) GetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Type
}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
func (o *AnalyticsResource) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Type, true
}

// SetType sets field value
func (o *AnalyticsResource) SetType(v string) {
	o.Type = v
}

func (o AnalyticsResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o AnalyticsResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Enabled) {
		toSerialize["enabled"] = o.Enabled
	}
	toSerialize["type"] = o.Type
	return toSerialize, nil
}

func (o *AnalyticsResource) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
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

	varAnalyticsResource := _AnalyticsResource{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varAnalyticsResource)

	if err != nil {
		return err
	}

	*o = AnalyticsResource(varAnalyticsResource)

	return err
}

type NullableAnalyticsResource struct {
	value *AnalyticsResource
	isSet bool
}

func (v NullableAnalyticsResource) Get() *AnalyticsResource {
	return v.value
}

func (v *NullableAnalyticsResource) Set(val *AnalyticsResource) {
	v.value = val
	v.isSet = true
}

func (v NullableAnalyticsResource) IsSet() bool {
	return v.isSet
}

func (v *NullableAnalyticsResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableAnalyticsResource(val *AnalyticsResource) *NullableAnalyticsResource {
	return &NullableAnalyticsResource{value: val, isSet: true}
}

func (v NullableAnalyticsResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableAnalyticsResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
