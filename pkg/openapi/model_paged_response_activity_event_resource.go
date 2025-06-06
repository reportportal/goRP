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

// checks if the PagedResponseActivityEventResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &PagedResponseActivityEventResource{}

// PagedResponseActivityEventResource struct for PagedResponseActivityEventResource
type PagedResponseActivityEventResource struct {
	Offset     int64                   `json:"offset"`
	Limit      int32                   `json:"limit"`
	TotalCount int64                   `json:"total_count"`
	Sort       string                  `json:"sort"`
	Order      string                  `json:"order"`
	Items      []ActivityEventResource `json:"items"`
}

type _PagedResponseActivityEventResource PagedResponseActivityEventResource

// NewPagedResponseActivityEventResource instantiates a new PagedResponseActivityEventResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewPagedResponseActivityEventResource(offset int64, limit int32, totalCount int64, sort string, order string, items []ActivityEventResource) *PagedResponseActivityEventResource {
	this := PagedResponseActivityEventResource{}
	this.Offset = offset
	this.Limit = limit
	this.TotalCount = totalCount
	this.Sort = sort
	this.Order = order
	this.Items = items
	return &this
}

// NewPagedResponseActivityEventResourceWithDefaults instantiates a new PagedResponseActivityEventResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewPagedResponseActivityEventResourceWithDefaults() *PagedResponseActivityEventResource {
	this := PagedResponseActivityEventResource{}
	return &this
}

// GetOffset returns the Offset field value
func (o *PagedResponseActivityEventResource) GetOffset() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Offset
}

// GetOffsetOk returns a tuple with the Offset field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetOffsetOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Offset, true
}

// SetOffset sets field value
func (o *PagedResponseActivityEventResource) SetOffset(v int64) {
	o.Offset = v
}

// GetLimit returns the Limit field value
func (o *PagedResponseActivityEventResource) GetLimit() int32 {
	if o == nil {
		var ret int32
		return ret
	}

	return o.Limit
}

// GetLimitOk returns a tuple with the Limit field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetLimitOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Limit, true
}

// SetLimit sets field value
func (o *PagedResponseActivityEventResource) SetLimit(v int32) {
	o.Limit = v
}

// GetTotalCount returns the TotalCount field value
func (o *PagedResponseActivityEventResource) GetTotalCount() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.TotalCount
}

// GetTotalCountOk returns a tuple with the TotalCount field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetTotalCountOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.TotalCount, true
}

// SetTotalCount sets field value
func (o *PagedResponseActivityEventResource) SetTotalCount(v int64) {
	o.TotalCount = v
}

// GetSort returns the Sort field value
func (o *PagedResponseActivityEventResource) GetSort() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Sort
}

// GetSortOk returns a tuple with the Sort field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetSortOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Sort, true
}

// SetSort sets field value
func (o *PagedResponseActivityEventResource) SetSort(v string) {
	o.Sort = v
}

// GetOrder returns the Order field value
func (o *PagedResponseActivityEventResource) GetOrder() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Order
}

// GetOrderOk returns a tuple with the Order field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetOrderOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Order, true
}

// SetOrder sets field value
func (o *PagedResponseActivityEventResource) SetOrder(v string) {
	o.Order = v
}

// GetItems returns the Items field value
func (o *PagedResponseActivityEventResource) GetItems() []ActivityEventResource {
	if o == nil {
		var ret []ActivityEventResource
		return ret
	}

	return o.Items
}

// GetItemsOk returns a tuple with the Items field value
// and a boolean to check if the value has been set.
func (o *PagedResponseActivityEventResource) GetItemsOk() ([]ActivityEventResource, bool) {
	if o == nil {
		return nil, false
	}
	return o.Items, true
}

// SetItems sets field value
func (o *PagedResponseActivityEventResource) SetItems(v []ActivityEventResource) {
	o.Items = v
}

func (o PagedResponseActivityEventResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o PagedResponseActivityEventResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["offset"] = o.Offset
	toSerialize["limit"] = o.Limit
	toSerialize["total_count"] = o.TotalCount
	toSerialize["sort"] = o.Sort
	toSerialize["order"] = o.Order
	toSerialize["items"] = o.Items
	return toSerialize, nil
}

func (o *PagedResponseActivityEventResource) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"offset",
		"limit",
		"total_count",
		"sort",
		"order",
		"items",
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

	varPagedResponseActivityEventResource := _PagedResponseActivityEventResource{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varPagedResponseActivityEventResource)

	if err != nil {
		return err
	}

	*o = PagedResponseActivityEventResource(varPagedResponseActivityEventResource)

	return err
}

type NullablePagedResponseActivityEventResource struct {
	value *PagedResponseActivityEventResource
	isSet bool
}

func (v NullablePagedResponseActivityEventResource) Get() *PagedResponseActivityEventResource {
	return v.value
}

func (v *NullablePagedResponseActivityEventResource) Set(val *PagedResponseActivityEventResource) {
	v.value = val
	v.isSet = true
}

func (v NullablePagedResponseActivityEventResource) IsSet() bool {
	return v.isSet
}

func (v *NullablePagedResponseActivityEventResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullablePagedResponseActivityEventResource(val *PagedResponseActivityEventResource) *NullablePagedResponseActivityEventResource {
	return &NullablePagedResponseActivityEventResource{value: val, isSet: true}
}

func (v NullablePagedResponseActivityEventResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullablePagedResponseActivityEventResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
