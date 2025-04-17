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

// checks if the BatchElementCreatedRS type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &BatchElementCreatedRS{}

// BatchElementCreatedRS struct for BatchElementCreatedRS
type BatchElementCreatedRS struct {
	Id         *string `json:"id,omitempty"`
	Message    *string `json:"message,omitempty"`
	StackTrace *string `json:"stackTrace,omitempty"`
}

// NewBatchElementCreatedRS instantiates a new BatchElementCreatedRS object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewBatchElementCreatedRS() *BatchElementCreatedRS {
	this := BatchElementCreatedRS{}
	return &this
}

// NewBatchElementCreatedRSWithDefaults instantiates a new BatchElementCreatedRS object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewBatchElementCreatedRSWithDefaults() *BatchElementCreatedRS {
	this := BatchElementCreatedRS{}
	return &this
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *BatchElementCreatedRS) GetId() string {
	if o == nil || IsNil(o.Id) {
		var ret string
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *BatchElementCreatedRS) GetIdOk() (*string, bool) {
	if o == nil || IsNil(o.Id) {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *BatchElementCreatedRS) HasId() bool {
	if o != nil && !IsNil(o.Id) {
		return true
	}

	return false
}

// SetId gets a reference to the given string and assigns it to the Id field.
func (o *BatchElementCreatedRS) SetId(v string) {
	o.Id = &v
}

// GetMessage returns the Message field value if set, zero value otherwise.
func (o *BatchElementCreatedRS) GetMessage() string {
	if o == nil || IsNil(o.Message) {
		var ret string
		return ret
	}
	return *o.Message
}

// GetMessageOk returns a tuple with the Message field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *BatchElementCreatedRS) GetMessageOk() (*string, bool) {
	if o == nil || IsNil(o.Message) {
		return nil, false
	}
	return o.Message, true
}

// HasMessage returns a boolean if a field has been set.
func (o *BatchElementCreatedRS) HasMessage() bool {
	if o != nil && !IsNil(o.Message) {
		return true
	}

	return false
}

// SetMessage gets a reference to the given string and assigns it to the Message field.
func (o *BatchElementCreatedRS) SetMessage(v string) {
	o.Message = &v
}

// GetStackTrace returns the StackTrace field value if set, zero value otherwise.
func (o *BatchElementCreatedRS) GetStackTrace() string {
	if o == nil || IsNil(o.StackTrace) {
		var ret string
		return ret
	}
	return *o.StackTrace
}

// GetStackTraceOk returns a tuple with the StackTrace field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *BatchElementCreatedRS) GetStackTraceOk() (*string, bool) {
	if o == nil || IsNil(o.StackTrace) {
		return nil, false
	}
	return o.StackTrace, true
}

// HasStackTrace returns a boolean if a field has been set.
func (o *BatchElementCreatedRS) HasStackTrace() bool {
	if o != nil && !IsNil(o.StackTrace) {
		return true
	}

	return false
}

// SetStackTrace gets a reference to the given string and assigns it to the StackTrace field.
func (o *BatchElementCreatedRS) SetStackTrace(v string) {
	o.StackTrace = &v
}

func (o BatchElementCreatedRS) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o BatchElementCreatedRS) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Id) {
		toSerialize["id"] = o.Id
	}
	if !IsNil(o.Message) {
		toSerialize["message"] = o.Message
	}
	if !IsNil(o.StackTrace) {
		toSerialize["stackTrace"] = o.StackTrace
	}
	return toSerialize, nil
}

type NullableBatchElementCreatedRS struct {
	value *BatchElementCreatedRS
	isSet bool
}

func (v NullableBatchElementCreatedRS) Get() *BatchElementCreatedRS {
	return v.value
}

func (v *NullableBatchElementCreatedRS) Set(val *BatchElementCreatedRS) {
	v.value = val
	v.isSet = true
}

func (v NullableBatchElementCreatedRS) IsSet() bool {
	return v.isSet
}

func (v *NullableBatchElementCreatedRS) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableBatchElementCreatedRS(val *BatchElementCreatedRS) *NullableBatchElementCreatedRS {
	return &NullableBatchElementCreatedRS{value: val, isSet: true}
}

func (v NullableBatchElementCreatedRS) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableBatchElementCreatedRS) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
