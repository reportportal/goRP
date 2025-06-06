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

// checks if the PageUserResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &PageUserResource{}

// PageUserResource struct for PageUserResource
type PageUserResource struct {
	Content []UserResource `json:"content,omitempty"`
	Page    *PageMetadata  `json:"page,omitempty"`
}

// NewPageUserResource instantiates a new PageUserResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewPageUserResource() *PageUserResource {
	this := PageUserResource{}
	return &this
}

// NewPageUserResourceWithDefaults instantiates a new PageUserResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewPageUserResourceWithDefaults() *PageUserResource {
	this := PageUserResource{}
	return &this
}

// GetContent returns the Content field value if set, zero value otherwise.
func (o *PageUserResource) GetContent() []UserResource {
	if o == nil || IsNil(o.Content) {
		var ret []UserResource
		return ret
	}
	return o.Content
}

// GetContentOk returns a tuple with the Content field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageUserResource) GetContentOk() ([]UserResource, bool) {
	if o == nil || IsNil(o.Content) {
		return nil, false
	}
	return o.Content, true
}

// HasContent returns a boolean if a field has been set.
func (o *PageUserResource) HasContent() bool {
	if o != nil && !IsNil(o.Content) {
		return true
	}

	return false
}

// SetContent gets a reference to the given []UserResource and assigns it to the Content field.
func (o *PageUserResource) SetContent(v []UserResource) {
	o.Content = v
}

// GetPage returns the Page field value if set, zero value otherwise.
func (o *PageUserResource) GetPage() PageMetadata {
	if o == nil || IsNil(o.Page) {
		var ret PageMetadata
		return ret
	}
	return *o.Page
}

// GetPageOk returns a tuple with the Page field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageUserResource) GetPageOk() (*PageMetadata, bool) {
	if o == nil || IsNil(o.Page) {
		return nil, false
	}
	return o.Page, true
}

// HasPage returns a boolean if a field has been set.
func (o *PageUserResource) HasPage() bool {
	if o != nil && !IsNil(o.Page) {
		return true
	}

	return false
}

// SetPage gets a reference to the given PageMetadata and assigns it to the Page field.
func (o *PageUserResource) SetPage(v PageMetadata) {
	o.Page = &v
}

func (o PageUserResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o PageUserResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Content) {
		toSerialize["content"] = o.Content
	}
	if !IsNil(o.Page) {
		toSerialize["page"] = o.Page
	}
	return toSerialize, nil
}

type NullablePageUserResource struct {
	value *PageUserResource
	isSet bool
}

func (v NullablePageUserResource) Get() *PageUserResource {
	return v.value
}

func (v *NullablePageUserResource) Set(val *PageUserResource) {
	v.value = val
	v.isSet = true
}

func (v NullablePageUserResource) IsSet() bool {
	return v.isSet
}

func (v *NullablePageUserResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullablePageUserResource(val *PageUserResource) *NullablePageUserResource {
	return &NullablePageUserResource{value: val, isSet: true}
}

func (v NullablePageUserResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullablePageUserResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
