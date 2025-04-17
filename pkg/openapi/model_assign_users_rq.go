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

// checks if the AssignUsersRQ type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &AssignUsersRQ{}

// AssignUsersRQ struct for AssignUsersRQ
type AssignUsersRQ struct {
	UserNames map[string]string `json:"userNames"`
}

type _AssignUsersRQ AssignUsersRQ

// NewAssignUsersRQ instantiates a new AssignUsersRQ object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewAssignUsersRQ(userNames map[string]string) *AssignUsersRQ {
	this := AssignUsersRQ{}
	this.UserNames = userNames
	return &this
}

// NewAssignUsersRQWithDefaults instantiates a new AssignUsersRQ object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewAssignUsersRQWithDefaults() *AssignUsersRQ {
	this := AssignUsersRQ{}
	return &this
}

// GetUserNames returns the UserNames field value
func (o *AssignUsersRQ) GetUserNames() map[string]string {
	if o == nil {
		var ret map[string]string
		return ret
	}

	return o.UserNames
}

// GetUserNamesOk returns a tuple with the UserNames field value
// and a boolean to check if the value has been set.
func (o *AssignUsersRQ) GetUserNamesOk() (map[string]string, bool) {
	if o == nil {
		return map[string]string{}, false
	}
	return o.UserNames, true
}

// SetUserNames sets field value
func (o *AssignUsersRQ) SetUserNames(v map[string]string) {
	o.UserNames = v
}

func (o AssignUsersRQ) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o AssignUsersRQ) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["userNames"] = o.UserNames
	return toSerialize, nil
}

func (o *AssignUsersRQ) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"userNames",
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

	varAssignUsersRQ := _AssignUsersRQ{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varAssignUsersRQ)

	if err != nil {
		return err
	}

	*o = AssignUsersRQ(varAssignUsersRQ)

	return err
}

type NullableAssignUsersRQ struct {
	value *AssignUsersRQ
	isSet bool
}

func (v NullableAssignUsersRQ) Get() *AssignUsersRQ {
	return v.value
}

func (v *NullableAssignUsersRQ) Set(val *AssignUsersRQ) {
	v.value = val
	v.isSet = true
}

func (v NullableAssignUsersRQ) IsSet() bool {
	return v.isSet
}

func (v *NullableAssignUsersRQ) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableAssignUsersRQ(val *AssignUsersRQ) *NullableAssignUsersRQ {
	return &NullableAssignUsersRQ{value: val, isSet: true}
}

func (v NullableAssignUsersRQ) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableAssignUsersRQ) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
