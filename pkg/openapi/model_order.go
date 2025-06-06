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

// checks if the Order type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &Order{}

// Order struct for Order
type Order struct {
	SortingColumn string `json:"sortingColumn"`
	IsAsc         bool   `json:"isAsc"`
}

type _Order Order

// NewOrder instantiates a new Order object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewOrder(sortingColumn string, isAsc bool) *Order {
	this := Order{}
	this.SortingColumn = sortingColumn
	this.IsAsc = isAsc
	return &this
}

// NewOrderWithDefaults instantiates a new Order object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewOrderWithDefaults() *Order {
	this := Order{}
	return &this
}

// GetSortingColumn returns the SortingColumn field value
func (o *Order) GetSortingColumn() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.SortingColumn
}

// GetSortingColumnOk returns a tuple with the SortingColumn field value
// and a boolean to check if the value has been set.
func (o *Order) GetSortingColumnOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.SortingColumn, true
}

// SetSortingColumn sets field value
func (o *Order) SetSortingColumn(v string) {
	o.SortingColumn = v
}

// GetIsAsc returns the IsAsc field value
func (o *Order) GetIsAsc() bool {
	if o == nil {
		var ret bool
		return ret
	}

	return o.IsAsc
}

// GetIsAscOk returns a tuple with the IsAsc field value
// and a boolean to check if the value has been set.
func (o *Order) GetIsAscOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}
	return &o.IsAsc, true
}

// SetIsAsc sets field value
func (o *Order) SetIsAsc(v bool) {
	o.IsAsc = v
}

func (o Order) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o Order) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["sortingColumn"] = o.SortingColumn
	toSerialize["isAsc"] = o.IsAsc
	return toSerialize, nil
}

func (o *Order) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"sortingColumn",
		"isAsc",
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

	varOrder := _Order{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varOrder)

	if err != nil {
		return err
	}

	*o = Order(varOrder)

	return err
}

type NullableOrder struct {
	value *Order
	isSet bool
}

func (v NullableOrder) Get() *Order {
	return v.value
}

func (v *NullableOrder) Set(val *Order) {
	v.value = val
	v.isSet = true
}

func (v NullableOrder) IsSet() bool {
	return v.isSet
}

func (v *NullableOrder) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableOrder(val *Order) *NullableOrder {
	return &NullableOrder{value: val, isSet: true}
}

func (v NullableOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableOrder) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
