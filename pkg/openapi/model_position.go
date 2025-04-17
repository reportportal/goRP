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

// checks if the Position type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &Position{}

// Position struct for Position
type Position struct {
	PositionX *int32 `json:"positionX,omitempty"`
	PositionY *int32 `json:"positionY,omitempty"`
}

// NewPosition instantiates a new Position object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewPosition() *Position {
	this := Position{}
	return &this
}

// NewPositionWithDefaults instantiates a new Position object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewPositionWithDefaults() *Position {
	this := Position{}
	return &this
}

// GetPositionX returns the PositionX field value if set, zero value otherwise.
func (o *Position) GetPositionX() int32 {
	if o == nil || IsNil(o.PositionX) {
		var ret int32
		return ret
	}
	return *o.PositionX
}

// GetPositionXOk returns a tuple with the PositionX field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Position) GetPositionXOk() (*int32, bool) {
	if o == nil || IsNil(o.PositionX) {
		return nil, false
	}
	return o.PositionX, true
}

// HasPositionX returns a boolean if a field has been set.
func (o *Position) HasPositionX() bool {
	if o != nil && !IsNil(o.PositionX) {
		return true
	}

	return false
}

// SetPositionX gets a reference to the given int32 and assigns it to the PositionX field.
func (o *Position) SetPositionX(v int32) {
	o.PositionX = &v
}

// GetPositionY returns the PositionY field value if set, zero value otherwise.
func (o *Position) GetPositionY() int32 {
	if o == nil || IsNil(o.PositionY) {
		var ret int32
		return ret
	}
	return *o.PositionY
}

// GetPositionYOk returns a tuple with the PositionY field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Position) GetPositionYOk() (*int32, bool) {
	if o == nil || IsNil(o.PositionY) {
		return nil, false
	}
	return o.PositionY, true
}

// HasPositionY returns a boolean if a field has been set.
func (o *Position) HasPositionY() bool {
	if o != nil && !IsNil(o.PositionY) {
		return true
	}

	return false
}

// SetPositionY gets a reference to the given int32 and assigns it to the PositionY field.
func (o *Position) SetPositionY(v int32) {
	o.PositionY = &v
}

func (o Position) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o Position) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.PositionX) {
		toSerialize["positionX"] = o.PositionX
	}
	if !IsNil(o.PositionY) {
		toSerialize["positionY"] = o.PositionY
	}
	return toSerialize, nil
}

type NullablePosition struct {
	value *Position
	isSet bool
}

func (v NullablePosition) Get() *Position {
	return v.value
}

func (v *NullablePosition) Set(val *Position) {
	v.value = val
	v.isSet = true
}

func (v NullablePosition) IsSet() bool {
	return v.isSet
}

func (v *NullablePosition) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullablePosition(val *Position) *NullablePosition {
	return &NullablePosition{value: val, isSet: true}
}

func (v NullablePosition) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullablePosition) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
