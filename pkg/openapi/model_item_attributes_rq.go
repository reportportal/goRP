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

// checks if the ItemAttributesRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ItemAttributesRQ{}

// ItemAttributesRQ struct for ItemAttributesRQ
type ItemAttributesRQ struct {
	Key    *string `json:"key,omitempty"`
	Value  string  `json:"value"`
	System *bool   `json:"system,omitempty"`
}

type _ItemAttributesRQ ItemAttributesRQ

// NewItemAttributesRQ instantiates a new ItemAttributesRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewItemAttributesRQ(value string) *ItemAttributesRQ {
	this := ItemAttributesRQ{}
	this.Value = value
	return &this
}

// NewItemAttributesRQWithDefaults instantiates a new ItemAttributesRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewItemAttributesRQWithDefaults() *ItemAttributesRQ {
	this := ItemAttributesRQ{}
	return &this
}

// GetKey returns the Key field value if set, zero value otherwise.
func (o *ItemAttributesRQ) GetKey() string {
	if o == nil || IsNil(o.Key) {
		var ret string
		return ret
	}
	return *o.Key
}

// GetKeyOk returns a tuple with the Key field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ItemAttributesRQ) GetKeyOk() (*string, bool) {
	if o == nil || IsNil(o.Key) {
		return nil, false
	}
	return o.Key, true
}

// HasKey returns a boolean if a field has been set.
func (o *ItemAttributesRQ) HasKey() bool {
	if o != nil && !IsNil(o.Key) {
		return true
	}

	return false
}

// SetKey gets a reference to the given string and assigns it to the Key field.
func (o *ItemAttributesRQ) SetKey(v string) {
	o.Key = &v
}

// GetValue returns the Value field value
func (o *ItemAttributesRQ) GetValue() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Value
}

// GetValueOk returns a tuple with the Value field value
// and a boolean to check if the value has been set.
func (o *ItemAttributesRQ) GetValueOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Value, true
}

// SetValue sets field value
func (o *ItemAttributesRQ) SetValue(v string) {
	o.Value = v
}

// GetSystem returns the System field value if set, zero value otherwise.
func (o *ItemAttributesRQ) GetSystem() bool {
	if o == nil || IsNil(o.System) {
		var ret bool
		return ret
	}
	return *o.System
}

// GetSystemOk returns a tuple with the System field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ItemAttributesRQ) GetSystemOk() (*bool, bool) {
	if o == nil || IsNil(o.System) {
		return nil, false
	}
	return o.System, true
}

// HasSystem returns a boolean if a field has been set.
func (o *ItemAttributesRQ) HasSystem() bool {
	if o != nil && !IsNil(o.System) {
		return true
	}

	return false
}

// SetSystem gets a reference to the given bool and assigns it to the System field.
func (o *ItemAttributesRQ) SetSystem(v bool) {
	o.System = &v
}

func (o ItemAttributesRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ItemAttributesRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Key) {
		toSerialize["key"] = o.Key
	}
	toSerialize["value"] = o.Value
	if !IsNil(o.System) {
		toSerialize["system"] = o.System
	}
	return toSerialize, nil
}

func (o *ItemAttributesRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
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

	varItemAttributesRQ := _ItemAttributesRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varItemAttributesRQ)

	if err != nil {
		return err
	}

	*o = ItemAttributesRQ(varItemAttributesRQ)

	return err
}

type NullableItemAttributesRQ struct {
	value *ItemAttributesRQ
	isSet bool
}

func (v NullableItemAttributesRQ) Get() *ItemAttributesRQ {
	return v.value
}

func (v *NullableItemAttributesRQ) Set(val *ItemAttributesRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableItemAttributesRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableItemAttributesRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableItemAttributesRQ(val *ItemAttributesRQ) *NullableItemAttributesRQ {
	return &NullableItemAttributesRQ{value: val, isSet: true}
}

func (v NullableItemAttributesRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableItemAttributesRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
