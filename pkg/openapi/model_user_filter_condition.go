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

// checks if the UserFilterCondition type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &UserFilterCondition{}

// UserFilterCondition struct for UserFilterCondition
type UserFilterCondition struct {
	FilteringField string `json:"filteringField"`
	Condition      string `json:"condition"`
	Value          string `json:"value"`
}

type _UserFilterCondition UserFilterCondition

// NewUserFilterCondition instantiates a new UserFilterCondition object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewUserFilterCondition(filteringField string, condition string, value string) *UserFilterCondition {
	this := UserFilterCondition{}
	this.FilteringField = filteringField
	this.Condition = condition
	this.Value = value
	return &this
}

// NewUserFilterConditionWithDefaults instantiates a new UserFilterCondition object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewUserFilterConditionWithDefaults() *UserFilterCondition {
	this := UserFilterCondition{}
	return &this
}

// GetFilteringField returns the FilteringField field value
func (o *UserFilterCondition) GetFilteringField() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.FilteringField
}

// GetFilteringFieldOk returns a tuple with the FilteringField field value
// and a boolean to check if the value has been set.
func (o *UserFilterCondition) GetFilteringFieldOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.FilteringField, true
}

// SetFilteringField sets field value
func (o *UserFilterCondition) SetFilteringField(v string) {
	o.FilteringField = v
}

// GetCondition returns the Condition field value
func (o *UserFilterCondition) GetCondition() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Condition
}

// GetConditionOk returns a tuple with the Condition field value
// and a boolean to check if the value has been set.
func (o *UserFilterCondition) GetConditionOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Condition, true
}

// SetCondition sets field value
func (o *UserFilterCondition) SetCondition(v string) {
	o.Condition = v
}

// GetValue returns the Value field value
func (o *UserFilterCondition) GetValue() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Value
}

// GetValueOk returns a tuple with the Value field value
// and a boolean to check if the value has been set.
func (o *UserFilterCondition) GetValueOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Value, true
}

// SetValue sets field value
func (o *UserFilterCondition) SetValue(v string) {
	o.Value = v
}

func (o UserFilterCondition) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o UserFilterCondition) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["filteringField"] = o.FilteringField
	toSerialize["condition"] = o.Condition
	toSerialize["value"] = o.Value
	return toSerialize, nil
}

func (o *UserFilterCondition) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"filteringField",
		"condition",
		"value",
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

	varUserFilterCondition := _UserFilterCondition{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varUserFilterCondition)

	if err != nil {
		return err
	}

	*o = UserFilterCondition(varUserFilterCondition)

	return err
}

type NullableUserFilterCondition struct {
	value *UserFilterCondition
	isSet bool
}

func (v NullableUserFilterCondition) Get() *UserFilterCondition {
	return v.value
}

func (v *NullableUserFilterCondition) Set(val *UserFilterCondition) {
	v.value = val
	v.isSet = true
}

func (v NullableUserFilterCondition) IsSet() bool {
	return v.isSet
}

func (v *NullableUserFilterCondition) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableUserFilterCondition(val *UserFilterCondition) *NullableUserFilterCondition {
	return &NullableUserFilterCondition{value: val, isSet: true}
}

func (v NullableUserFilterCondition) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableUserFilterCondition) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
