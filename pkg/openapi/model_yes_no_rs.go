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

// checks if the YesNoRS type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &YesNoRS{}

// YesNoRS struct for YesNoRS
type YesNoRS struct {
	Is *bool `json:"is,omitempty"`
}

// NewYesNoRS instantiates a new YesNoRS object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewYesNoRS() *YesNoRS {
	this := YesNoRS{}
	return &this
}

// NewYesNoRSWithDefaults instantiates a new YesNoRS object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewYesNoRSWithDefaults() *YesNoRS {
	this := YesNoRS{}
	return &this
}

// GetIs returns the Is field value if set, zero value otherwise.
func (o *YesNoRS) GetIs() bool {
	if o == nil || IsNil(o.Is) {
		var ret bool
		return ret
	}
	return *o.Is
}

// GetIsOk returns a tuple with the Is field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *YesNoRS) GetIsOk() (*bool, bool) {
	if o == nil || IsNil(o.Is) {
		return nil, false
	}
	return o.Is, true
}

// HasIs returns a boolean if a field has been set.
func (o *YesNoRS) HasIs() bool {
	if o != nil && !IsNil(o.Is) {
		return true
	}

	return false
}

// SetIs gets a reference to the given bool and assigns it to the Is field.
func (o *YesNoRS) SetIs(v bool) {
	o.Is = &v
}

func (o YesNoRS) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o YesNoRS) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Is) {
		toSerialize["is"] = o.Is
	}
	return toSerialize, nil
}

type NullableYesNoRS struct {
	value *YesNoRS
	isSet bool
}

func (v NullableYesNoRS) Get() *YesNoRS {
	return v.value
}

func (v *NullableYesNoRS) Set(val *YesNoRS) {
	v.value = val
	v.isSet = true
}

func (v NullableYesNoRS) IsSet() bool {
	return v.isSet
}

func (v *NullableYesNoRS) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableYesNoRS(val *YesNoRS) *NullableYesNoRS {
	return &NullableYesNoRS{value: val, isSet: true}
}

func (v NullableYesNoRS) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableYesNoRS) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
