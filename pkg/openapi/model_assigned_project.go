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

// checks if the AssignedProject type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &AssignedProject{}

// AssignedProject struct for AssignedProject
type AssignedProject struct {
	ProjectRole *string `json:"projectRole,omitempty"`
	EntryType   *string `json:"entryType,omitempty"`
}

// NewAssignedProject instantiates a new AssignedProject object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewAssignedProject() *AssignedProject {
	this := AssignedProject{}
	return &this
}

// NewAssignedProjectWithDefaults instantiates a new AssignedProject object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewAssignedProjectWithDefaults() *AssignedProject {
	this := AssignedProject{}
	return &this
}

// GetProjectRole returns the ProjectRole field value if set, zero value otherwise.
func (o *AssignedProject) GetProjectRole() string {
	if o == nil || IsNil(o.ProjectRole) {
		var ret string
		return ret
	}
	return *o.ProjectRole
}

// GetProjectRoleOk returns a tuple with the ProjectRole field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AssignedProject) GetProjectRoleOk() (*string, bool) {
	if o == nil || IsNil(o.ProjectRole) {
		return nil, false
	}
	return o.ProjectRole, true
}

// HasProjectRole returns a boolean if a field has been set.
func (o *AssignedProject) HasProjectRole() bool {
	if o != nil && !IsNil(o.ProjectRole) {
		return true
	}

	return false
}

// SetProjectRole gets a reference to the given string and assigns it to the ProjectRole field.
func (o *AssignedProject) SetProjectRole(v string) {
	o.ProjectRole = &v
}

// GetEntryType returns the EntryType field value if set, zero value otherwise.
func (o *AssignedProject) GetEntryType() string {
	if o == nil || IsNil(o.EntryType) {
		var ret string
		return ret
	}
	return *o.EntryType
}

// GetEntryTypeOk returns a tuple with the EntryType field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AssignedProject) GetEntryTypeOk() (*string, bool) {
	if o == nil || IsNil(o.EntryType) {
		return nil, false
	}
	return o.EntryType, true
}

// HasEntryType returns a boolean if a field has been set.
func (o *AssignedProject) HasEntryType() bool {
	if o != nil && !IsNil(o.EntryType) {
		return true
	}

	return false
}

// SetEntryType gets a reference to the given string and assigns it to the EntryType field.
func (o *AssignedProject) SetEntryType(v string) {
	o.EntryType = &v
}

func (o AssignedProject) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o AssignedProject) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.ProjectRole) {
		toSerialize["projectRole"] = o.ProjectRole
	}
	if !IsNil(o.EntryType) {
		toSerialize["entryType"] = o.EntryType
	}
	return toSerialize, nil
}

type NullableAssignedProject struct {
	value *AssignedProject
	isSet bool
}

func (v NullableAssignedProject) Get() *AssignedProject {
	return v.value
}

func (v *NullableAssignedProject) Set(val *AssignedProject) {
	v.value = val
	v.isSet = true
}

func (v NullableAssignedProject) IsSet() bool {
	return v.isSet
}

func (v *NullableAssignedProject) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableAssignedProject(val *AssignedProject) *NullableAssignedProject {
	return &NullableAssignedProject{value: val, isSet: true}
}

func (v NullableAssignedProject) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableAssignedProject) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
