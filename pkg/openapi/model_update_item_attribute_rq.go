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

// checks if the UpdateItemAttributeRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &UpdateItemAttributeRQ{}

// UpdateItemAttributeRQ struct for UpdateItemAttributeRQ
type UpdateItemAttributeRQ struct {
	From   *ItemAttributeResource `json:"from,omitempty"`
	To     *ItemAttributeResource `json:"to,omitempty"`
	Action *string                `json:"action,omitempty"`
}

// NewUpdateItemAttributeRQ instantiates a new UpdateItemAttributeRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewUpdateItemAttributeRQ() *UpdateItemAttributeRQ {
	this := UpdateItemAttributeRQ{}
	return &this
}

// NewUpdateItemAttributeRQWithDefaults instantiates a new UpdateItemAttributeRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewUpdateItemAttributeRQWithDefaults() *UpdateItemAttributeRQ {
	this := UpdateItemAttributeRQ{}
	return &this
}

// GetFrom returns the From field value if set, zero value otherwise.
func (o *UpdateItemAttributeRQ) GetFrom() ItemAttributeResource {
	if o == nil || IsNil(o.From) {
		var ret ItemAttributeResource
		return ret
	}
	return *o.From
}

// GetFromOk returns a tuple with the From field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UpdateItemAttributeRQ) GetFromOk() (*ItemAttributeResource, bool) {
	if o == nil || IsNil(o.From) {
		return nil, false
	}
	return o.From, true
}

// HasFrom returns a boolean if a field has been set.
func (o *UpdateItemAttributeRQ) HasFrom() bool {
	if o != nil && !IsNil(o.From) {
		return true
	}

	return false
}

// SetFrom gets a reference to the given ItemAttributeResource and assigns it to the From field.
func (o *UpdateItemAttributeRQ) SetFrom(v ItemAttributeResource) {
	o.From = &v
}

// GetTo returns the To field value if set, zero value otherwise.
func (o *UpdateItemAttributeRQ) GetTo() ItemAttributeResource {
	if o == nil || IsNil(o.To) {
		var ret ItemAttributeResource
		return ret
	}
	return *o.To
}

// GetToOk returns a tuple with the To field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UpdateItemAttributeRQ) GetToOk() (*ItemAttributeResource, bool) {
	if o == nil || IsNil(o.To) {
		return nil, false
	}
	return o.To, true
}

// HasTo returns a boolean if a field has been set.
func (o *UpdateItemAttributeRQ) HasTo() bool {
	if o != nil && !IsNil(o.To) {
		return true
	}

	return false
}

// SetTo gets a reference to the given ItemAttributeResource and assigns it to the To field.
func (o *UpdateItemAttributeRQ) SetTo(v ItemAttributeResource) {
	o.To = &v
}

// GetAction returns the Action field value if set, zero value otherwise.
func (o *UpdateItemAttributeRQ) GetAction() string {
	if o == nil || IsNil(o.Action) {
		var ret string
		return ret
	}
	return *o.Action
}

// GetActionOk returns a tuple with the Action field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UpdateItemAttributeRQ) GetActionOk() (*string, bool) {
	if o == nil || IsNil(o.Action) {
		return nil, false
	}
	return o.Action, true
}

// HasAction returns a boolean if a field has been set.
func (o *UpdateItemAttributeRQ) HasAction() bool {
	if o != nil && !IsNil(o.Action) {
		return true
	}

	return false
}

// SetAction gets a reference to the given string and assigns it to the Action field.
func (o *UpdateItemAttributeRQ) SetAction(v string) {
	o.Action = &v
}

func (o UpdateItemAttributeRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o UpdateItemAttributeRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.From) {
		toSerialize["from"] = o.From
	}
	if !IsNil(o.To) {
		toSerialize["to"] = o.To
	}
	if !IsNil(o.Action) {
		toSerialize["action"] = o.Action
	}
	return toSerialize, nil
}

type NullableUpdateItemAttributeRQ struct {
	value *UpdateItemAttributeRQ
	isSet bool
}

func (v NullableUpdateItemAttributeRQ) Get() *UpdateItemAttributeRQ {
	return v.value
}

func (v *NullableUpdateItemAttributeRQ) Set(val *UpdateItemAttributeRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableUpdateItemAttributeRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableUpdateItemAttributeRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableUpdateItemAttributeRQ(val *UpdateItemAttributeRQ) *NullableUpdateItemAttributeRQ {
	return &NullableUpdateItemAttributeRQ{value: val, isSet: true}
}

func (v NullableUpdateItemAttributeRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableUpdateItemAttributeRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
