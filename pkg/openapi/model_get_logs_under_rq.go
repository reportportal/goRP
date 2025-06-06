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

// checks if the GetLogsUnderRq type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &GetLogsUnderRq{}

// GetLogsUnderRq struct for GetLogsUnderRq
type GetLogsUnderRq struct {
	ItemIds  []int64 `json:"itemIds"`
	LogLevel string  `json:"logLevel"`
}

type _GetLogsUnderRq GetLogsUnderRq

// NewGetLogsUnderRq instantiates a new GetLogsUnderRq object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewGetLogsUnderRq(itemIds []int64, logLevel string) *GetLogsUnderRq {
	this := GetLogsUnderRq{}
	this.ItemIds = itemIds
	this.LogLevel = logLevel
	return &this
}

// NewGetLogsUnderRqWithDefaults instantiates a new GetLogsUnderRq object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewGetLogsUnderRqWithDefaults() *GetLogsUnderRq {
	this := GetLogsUnderRq{}
	return &this
}

// GetItemIds returns the ItemIds field value
func (o *GetLogsUnderRq) GetItemIds() []int64 {
	if o == nil {
		var ret []int64
		return ret
	}

	return o.ItemIds
}

// GetItemIdsOk returns a tuple with the ItemIds field value
// and a boolean to check if the value has been set.
func (o *GetLogsUnderRq) GetItemIdsOk() ([]int64, bool) {
	if o == nil {
		return nil, false
	}
	return o.ItemIds, true
}

// SetItemIds sets field value
func (o *GetLogsUnderRq) SetItemIds(v []int64) {
	o.ItemIds = v
}

// GetLogLevel returns the LogLevel field value
func (o *GetLogsUnderRq) GetLogLevel() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.LogLevel
}

// GetLogLevelOk returns a tuple with the LogLevel field value
// and a boolean to check if the value has been set.
func (o *GetLogsUnderRq) GetLogLevelOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.LogLevel, true
}

// SetLogLevel sets field value
func (o *GetLogsUnderRq) SetLogLevel(v string) {
	o.LogLevel = v
}

func (o GetLogsUnderRq) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o GetLogsUnderRq) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["itemIds"] = o.ItemIds
	toSerialize["logLevel"] = o.LogLevel
	return toSerialize, nil
}

func (o *GetLogsUnderRq) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"itemIds",
		"logLevel",
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

	varGetLogsUnderRq := _GetLogsUnderRq{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varGetLogsUnderRq)

	if err != nil {
		return err
	}

	*o = GetLogsUnderRq(varGetLogsUnderRq)

	return err
}

type NullableGetLogsUnderRq struct {
	value *GetLogsUnderRq
	isSet bool
}

func (v NullableGetLogsUnderRq) Get() *GetLogsUnderRq {
	return v.value
}

func (v *NullableGetLogsUnderRq) Set(val *GetLogsUnderRq) {
	v.value = val
	v.isSet = true
}

func (v NullableGetLogsUnderRq) IsSet() bool {
	return v.isSet
}

func (v *NullableGetLogsUnderRq) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableGetLogsUnderRq(val *GetLogsUnderRq) *NullableGetLogsUnderRq {
	return &NullableGetLogsUnderRq{value: val, isSet: true}
}

func (v NullableGetLogsUnderRq) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableGetLogsUnderRq) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
