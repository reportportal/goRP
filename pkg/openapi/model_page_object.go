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

// checks if the PageObject type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &PageObject{}

// PageObject struct for PageObject
type PageObject struct {
	Content []map[string]interface{} `json:"content,omitempty"`
	Page    *PageMetadata            `json:"page,omitempty"`
}

// NewPageObject instantiates a new PageObject object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewPageObject() *PageObject {
	this := PageObject{}
	return &this
}

// NewPageObjectWithDefaults instantiates a new PageObject object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewPageObjectWithDefaults() *PageObject {
	this := PageObject{}
	return &this
}

// GetContent returns the Content field value if set, zero value otherwise.
func (o *PageObject) GetContent() []map[string]interface{} {
	if o == nil || IsNil(o.Content) {
		var ret []map[string]interface{}
		return ret
	}
	return o.Content
}

// GetContentOk returns a tuple with the Content field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageObject) GetContentOk() ([]map[string]interface{}, bool) {
	if o == nil || IsNil(o.Content) {
		return nil, false
	}
	return o.Content, true
}

// HasContent returns a boolean if a field has been set.
func (o *PageObject) HasContent() bool {
	if o != nil && !IsNil(o.Content) {
		return true
	}

	return false
}

// SetContent gets a reference to the given []map[string]interface{} and assigns it to the Content field.
func (o *PageObject) SetContent(v []map[string]interface{}) {
	o.Content = v
}

// GetPage returns the Page field value if set, zero value otherwise.
func (o *PageObject) GetPage() PageMetadata {
	if o == nil || IsNil(o.Page) {
		var ret PageMetadata
		return ret
	}
	return *o.Page
}

// GetPageOk returns a tuple with the Page field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageObject) GetPageOk() (*PageMetadata, bool) {
	if o == nil || IsNil(o.Page) {
		return nil, false
	}
	return o.Page, true
}

// HasPage returns a boolean if a field has been set.
func (o *PageObject) HasPage() bool {
	if o != nil && !IsNil(o.Page) {
		return true
	}

	return false
}

// SetPage gets a reference to the given PageMetadata and assigns it to the Page field.
func (o *PageObject) SetPage(v PageMetadata) {
	o.Page = &v
}

func (o PageObject) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o PageObject) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Content) {
		toSerialize["content"] = o.Content
	}
	if !IsNil(o.Page) {
		toSerialize["page"] = o.Page
	}
	return toSerialize, nil
}

type NullablePageObject struct {
	value *PageObject
	isSet bool
}

func (v NullablePageObject) Get() *PageObject {
	return v.value
}

func (v *NullablePageObject) Set(val *PageObject) {
	v.value = val
	v.isSet = true
}

func (v NullablePageObject) IsSet() bool {
	return v.isSet
}

func (v *NullablePageObject) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullablePageObject(val *PageObject) *NullablePageObject {
	return &NullablePageObject{value: val, isSet: true}
}

func (v NullablePageObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullablePageObject) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
