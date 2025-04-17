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

// checks if the PageLaunchResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &PageLaunchResource{}

// PageLaunchResource struct for PageLaunchResource
type PageLaunchResource struct {
	Content []LaunchResource `json:"content,omitempty"`
	Page    *PageMetadata    `json:"page,omitempty"`
}

// NewPageLaunchResource instantiates a new PageLaunchResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewPageLaunchResource() *PageLaunchResource {
	this := PageLaunchResource{}
	return &this
}

// NewPageLaunchResourceWithDefaults instantiates a new PageLaunchResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewPageLaunchResourceWithDefaults() *PageLaunchResource {
	this := PageLaunchResource{}
	return &this
}

// GetContent returns the Content field value if set, zero value otherwise.
func (o *PageLaunchResource) GetContent() []LaunchResource {
	if o == nil || IsNil(o.Content) {
		var ret []LaunchResource
		return ret
	}
	return o.Content
}

// GetContentOk returns a tuple with the Content field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageLaunchResource) GetContentOk() ([]LaunchResource, bool) {
	if o == nil || IsNil(o.Content) {
		return nil, false
	}
	return o.Content, true
}

// HasContent returns a boolean if a field has been set.
func (o *PageLaunchResource) HasContent() bool {
	if o != nil && !IsNil(o.Content) {
		return true
	}

	return false
}

// SetContent gets a reference to the given []LaunchResource and assigns it to the Content field.
func (o *PageLaunchResource) SetContent(v []LaunchResource) {
	o.Content = v
}

// GetPage returns the Page field value if set, zero value otherwise.
func (o *PageLaunchResource) GetPage() PageMetadata {
	if o == nil || IsNil(o.Page) {
		var ret PageMetadata
		return ret
	}
	return *o.Page
}

// GetPageOk returns a tuple with the Page field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *PageLaunchResource) GetPageOk() (*PageMetadata, bool) {
	if o == nil || IsNil(o.Page) {
		return nil, false
	}
	return o.Page, true
}

// HasPage returns a boolean if a field has been set.
func (o *PageLaunchResource) HasPage() bool {
	if o != nil && !IsNil(o.Page) {
		return true
	}

	return false
}

// SetPage gets a reference to the given PageMetadata and assigns it to the Page field.
func (o *PageLaunchResource) SetPage(v PageMetadata) {
	o.Page = &v
}

func (o PageLaunchResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o PageLaunchResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Content) {
		toSerialize["content"] = o.Content
	}
	if !IsNil(o.Page) {
		toSerialize["page"] = o.Page
	}
	return toSerialize, nil
}

type NullablePageLaunchResource struct {
	value *PageLaunchResource
	isSet bool
}

func (v NullablePageLaunchResource) Get() *PageLaunchResource {
	return v.value
}

func (v *NullablePageLaunchResource) Set(val *PageLaunchResource) {
	v.value = val
	v.isSet = true
}

func (v NullablePageLaunchResource) IsSet() bool {
	return v.isSet
}

func (v *NullablePageLaunchResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullablePageLaunchResource(val *PageLaunchResource) *NullablePageLaunchResource {
	return &NullablePageLaunchResource{value: val, isSet: true}
}

func (v NullablePageLaunchResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullablePageLaunchResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
