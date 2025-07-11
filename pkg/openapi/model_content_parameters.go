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

// checks if the ContentParameters type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ContentParameters{}

// ContentParameters struct for ContentParameters
type ContentParameters struct {
	ContentFields []string               `json:"contentFields,omitempty"`
	ItemsCount    *int32                 `json:"itemsCount,omitempty"`
	WidgetOptions map[string]interface{} `json:"widgetOptions,omitempty"`
}

// NewContentParameters instantiates a new ContentParameters object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewContentParameters() *ContentParameters {
	this := ContentParameters{}
	return &this
}

// NewContentParametersWithDefaults instantiates a new ContentParameters object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewContentParametersWithDefaults() *ContentParameters {
	this := ContentParameters{}
	return &this
}

// GetContentFields returns the ContentFields field value if set, zero value otherwise.
func (o *ContentParameters) GetContentFields() []string {
	if o == nil || IsNil(o.ContentFields) {
		var ret []string
		return ret
	}
	return o.ContentFields
}

// GetContentFieldsOk returns a tuple with the ContentFields field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ContentParameters) GetContentFieldsOk() ([]string, bool) {
	if o == nil || IsNil(o.ContentFields) {
		return nil, false
	}
	return o.ContentFields, true
}

// HasContentFields returns a boolean if a field has been set.
func (o *ContentParameters) HasContentFields() bool {
	if o != nil && !IsNil(o.ContentFields) {
		return true
	}

	return false
}

// SetContentFields gets a reference to the given []string and assigns it to the ContentFields field.
func (o *ContentParameters) SetContentFields(v []string) {
	o.ContentFields = v
}

// GetItemsCount returns the ItemsCount field value if set, zero value otherwise.
func (o *ContentParameters) GetItemsCount() int32 {
	if o == nil || IsNil(o.ItemsCount) {
		var ret int32
		return ret
	}
	return *o.ItemsCount
}

// GetItemsCountOk returns a tuple with the ItemsCount field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ContentParameters) GetItemsCountOk() (*int32, bool) {
	if o == nil || IsNil(o.ItemsCount) {
		return nil, false
	}
	return o.ItemsCount, true
}

// HasItemsCount returns a boolean if a field has been set.
func (o *ContentParameters) HasItemsCount() bool {
	if o != nil && !IsNil(o.ItemsCount) {
		return true
	}

	return false
}

// SetItemsCount gets a reference to the given int32 and assigns it to the ItemsCount field.
func (o *ContentParameters) SetItemsCount(v int32) {
	o.ItemsCount = &v
}

// GetWidgetOptions returns the WidgetOptions field value if set, zero value otherwise.
func (o *ContentParameters) GetWidgetOptions() map[string]interface{} {
	if o == nil || IsNil(o.WidgetOptions) {
		var ret map[string]interface{}
		return ret
	}
	return o.WidgetOptions
}

// GetWidgetOptionsOk returns a tuple with the WidgetOptions field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ContentParameters) GetWidgetOptionsOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.WidgetOptions) {
		return map[string]interface{}{}, false
	}
	return o.WidgetOptions, true
}

// HasWidgetOptions returns a boolean if a field has been set.
func (o *ContentParameters) HasWidgetOptions() bool {
	if o != nil && !IsNil(o.WidgetOptions) {
		return true
	}

	return false
}

// SetWidgetOptions gets a reference to the given map[string]interface{} and assigns it to the WidgetOptions field.
func (o *ContentParameters) SetWidgetOptions(v map[string]interface{}) {
	o.WidgetOptions = v
}

func (o ContentParameters) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ContentParameters) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.ContentFields) {
		toSerialize["contentFields"] = o.ContentFields
	}
	if !IsNil(o.ItemsCount) {
		toSerialize["itemsCount"] = o.ItemsCount
	}
	if !IsNil(o.WidgetOptions) {
		toSerialize["widgetOptions"] = o.WidgetOptions
	}
	return toSerialize, nil
}

type NullableContentParameters struct {
	value *ContentParameters
	isSet bool
}

func (v NullableContentParameters) Get() *ContentParameters {
	return v.value
}

func (v *NullableContentParameters) Set(val *ContentParameters) {
	v.value = val
	v.isSet = true
}

func (v NullableContentParameters) IsSet() bool {
	return v.isSet
}

func (v *NullableContentParameters) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableContentParameters(val *ContentParameters) *NullableContentParameters {
	return &NullableContentParameters{value: val, isSet: true}
}

func (v NullableContentParameters) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableContentParameters) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
