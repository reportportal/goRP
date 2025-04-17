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

// checks if the CreateClustersRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &CreateClustersRQ{}

// CreateClustersRQ struct for CreateClustersRQ
type CreateClustersRQ struct {
	LaunchId      int64 `json:"launchId"`
	RemoveNumbers *bool `json:"removeNumbers,omitempty"`
}

type _CreateClustersRQ CreateClustersRQ

// NewCreateClustersRQ instantiates a new CreateClustersRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewCreateClustersRQ(launchId int64) *CreateClustersRQ {
	this := CreateClustersRQ{}
	this.LaunchId = launchId
	return &this
}

// NewCreateClustersRQWithDefaults instantiates a new CreateClustersRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewCreateClustersRQWithDefaults() *CreateClustersRQ {
	this := CreateClustersRQ{}
	return &this
}

// GetLaunchId returns the LaunchId field value
func (o *CreateClustersRQ) GetLaunchId() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.LaunchId
}

// GetLaunchIdOk returns a tuple with the LaunchId field value
// and a boolean to check if the value has been set.
func (o *CreateClustersRQ) GetLaunchIdOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.LaunchId, true
}

// SetLaunchId sets field value
func (o *CreateClustersRQ) SetLaunchId(v int64) {
	o.LaunchId = v
}

// GetRemoveNumbers returns the RemoveNumbers field value if set, zero value otherwise.
func (o *CreateClustersRQ) GetRemoveNumbers() bool {
	if o == nil || IsNil(o.RemoveNumbers) {
		var ret bool
		return ret
	}
	return *o.RemoveNumbers
}

// GetRemoveNumbersOk returns a tuple with the RemoveNumbers field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateClustersRQ) GetRemoveNumbersOk() (*bool, bool) {
	if o == nil || IsNil(o.RemoveNumbers) {
		return nil, false
	}
	return o.RemoveNumbers, true
}

// HasRemoveNumbers returns a boolean if a field has been set.
func (o *CreateClustersRQ) HasRemoveNumbers() bool {
	if o != nil && !IsNil(o.RemoveNumbers) {
		return true
	}

	return false
}

// SetRemoveNumbers gets a reference to the given bool and assigns it to the RemoveNumbers field.
func (o *CreateClustersRQ) SetRemoveNumbers(v bool) {
	o.RemoveNumbers = &v
}

func (o CreateClustersRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o CreateClustersRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["launchId"] = o.LaunchId
	if !IsNil(o.RemoveNumbers) {
		toSerialize["removeNumbers"] = o.RemoveNumbers
	}
	return toSerialize, nil
}

func (o *CreateClustersRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"launchId",
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

	varCreateClustersRQ := _CreateClustersRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varCreateClustersRQ)

	if err != nil {
		return err
	}

	*o = CreateClustersRQ(varCreateClustersRQ)

	return err
}

type NullableCreateClustersRQ struct {
	value *CreateClustersRQ
	isSet bool
}

func (v NullableCreateClustersRQ) Get() *CreateClustersRQ {
	return v.value
}

func (v *NullableCreateClustersRQ) Set(val *CreateClustersRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableCreateClustersRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableCreateClustersRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableCreateClustersRQ(val *CreateClustersRQ) *NullableCreateClustersRQ {
	return &NullableCreateClustersRQ{value: val, isSet: true}
}

func (v NullableCreateClustersRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableCreateClustersRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
