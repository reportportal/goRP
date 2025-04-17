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
	"time"
)

// checks if the ProjectGroupInfo type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ProjectGroupInfo{}

// ProjectGroupInfo Information about project group.
type ProjectGroupInfo struct {
	// Resource identifier.
	Id *int64 `json:"id,omitempty"`
	// Group identifier for external usage.
	Uuid *string `json:"uuid,omitempty"`
	// Display name.
	Name *string `json:"name,omitempty" validate:"regexp=^[A-Za-z0-9.'_\\\\- ]+$"`
	// A slug is used to identify a resource. It should be unique and contain only lowercase letters, numbers, and hyphens. It should not start or end with a hyphen.
	Slug       *string `json:"slug,omitempty" validate:"regexp=^[a-z0-9]+(?:-[a-z0-9]+)*$"`
	Permission *string `json:"permission,omitempty"`
	// Timestamp of project adding to group.
	AddedAt *time.Time `json:"added_at,omitempty"`
	// Timestamp of project updating in group.
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// NewProjectGroupInfo instantiates a new ProjectGroupInfo object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewProjectGroupInfo() *ProjectGroupInfo {
	this := ProjectGroupInfo{}
	return &this
}

// NewProjectGroupInfoWithDefaults instantiates a new ProjectGroupInfo object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewProjectGroupInfoWithDefaults() *ProjectGroupInfo {
	this := ProjectGroupInfo{}
	return &this
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetId() int64 {
	if o == nil || IsNil(o.Id) {
		var ret int64
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetIdOk() (*int64, bool) {
	if o == nil || IsNil(o.Id) {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasId() bool {
	if o != nil && !IsNil(o.Id) {
		return true
	}

	return false
}

// SetId gets a reference to the given int64 and assigns it to the Id field.
func (o *ProjectGroupInfo) SetId(v int64) {
	o.Id = &v
}

// GetUuid returns the Uuid field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetUuid() string {
	if o == nil || IsNil(o.Uuid) {
		var ret string
		return ret
	}
	return *o.Uuid
}

// GetUuidOk returns a tuple with the Uuid field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetUuidOk() (*string, bool) {
	if o == nil || IsNil(o.Uuid) {
		return nil, false
	}
	return o.Uuid, true
}

// HasUuid returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasUuid() bool {
	if o != nil && !IsNil(o.Uuid) {
		return true
	}

	return false
}

// SetUuid gets a reference to the given string and assigns it to the Uuid field.
func (o *ProjectGroupInfo) SetUuid(v string) {
	o.Uuid = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetName() string {
	if o == nil || IsNil(o.Name) {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetNameOk() (*string, bool) {
	if o == nil || IsNil(o.Name) {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasName() bool {
	if o != nil && !IsNil(o.Name) {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *ProjectGroupInfo) SetName(v string) {
	o.Name = &v
}

// GetSlug returns the Slug field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetSlug() string {
	if o == nil || IsNil(o.Slug) {
		var ret string
		return ret
	}
	return *o.Slug
}

// GetSlugOk returns a tuple with the Slug field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetSlugOk() (*string, bool) {
	if o == nil || IsNil(o.Slug) {
		return nil, false
	}
	return o.Slug, true
}

// HasSlug returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasSlug() bool {
	if o != nil && !IsNil(o.Slug) {
		return true
	}

	return false
}

// SetSlug gets a reference to the given string and assigns it to the Slug field.
func (o *ProjectGroupInfo) SetSlug(v string) {
	o.Slug = &v
}

// GetPermission returns the Permission field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetPermission() string {
	if o == nil || IsNil(o.Permission) {
		var ret string
		return ret
	}
	return *o.Permission
}

// GetPermissionOk returns a tuple with the Permission field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetPermissionOk() (*string, bool) {
	if o == nil || IsNil(o.Permission) {
		return nil, false
	}
	return o.Permission, true
}

// HasPermission returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasPermission() bool {
	if o != nil && !IsNil(o.Permission) {
		return true
	}

	return false
}

// SetPermission gets a reference to the given string and assigns it to the Permission field.
func (o *ProjectGroupInfo) SetPermission(v string) {
	o.Permission = &v
}

// GetAddedAt returns the AddedAt field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetAddedAt() time.Time {
	if o == nil || IsNil(o.AddedAt) {
		var ret time.Time
		return ret
	}
	return *o.AddedAt
}

// GetAddedAtOk returns a tuple with the AddedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetAddedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.AddedAt) {
		return nil, false
	}
	return o.AddedAt, true
}

// HasAddedAt returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasAddedAt() bool {
	if o != nil && !IsNil(o.AddedAt) {
		return true
	}

	return false
}

// SetAddedAt gets a reference to the given time.Time and assigns it to the AddedAt field.
func (o *ProjectGroupInfo) SetAddedAt(v time.Time) {
	o.AddedAt = &v
}

// GetUpdatedAt returns the UpdatedAt field value if set, zero value otherwise.
func (o *ProjectGroupInfo) GetUpdatedAt() time.Time {
	if o == nil || IsNil(o.UpdatedAt) {
		var ret time.Time
		return ret
	}
	return *o.UpdatedAt
}

// GetUpdatedAtOk returns a tuple with the UpdatedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ProjectGroupInfo) GetUpdatedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.UpdatedAt) {
		return nil, false
	}
	return o.UpdatedAt, true
}

// HasUpdatedAt returns a boolean if a field has been set.
func (o *ProjectGroupInfo) HasUpdatedAt() bool {
	if o != nil && !IsNil(o.UpdatedAt) {
		return true
	}

	return false
}

// SetUpdatedAt gets a reference to the given time.Time and assigns it to the UpdatedAt field.
func (o *ProjectGroupInfo) SetUpdatedAt(v time.Time) {
	o.UpdatedAt = &v
}

func (o ProjectGroupInfo) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ProjectGroupInfo) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Id) {
		toSerialize["id"] = o.Id
	}
	if !IsNil(o.Uuid) {
		toSerialize["uuid"] = o.Uuid
	}
	if !IsNil(o.Name) {
		toSerialize["name"] = o.Name
	}
	if !IsNil(o.Slug) {
		toSerialize["slug"] = o.Slug
	}
	if !IsNil(o.Permission) {
		toSerialize["permission"] = o.Permission
	}
	if !IsNil(o.AddedAt) {
		toSerialize["added_at"] = o.AddedAt
	}
	if !IsNil(o.UpdatedAt) {
		toSerialize["updated_at"] = o.UpdatedAt
	}
	return toSerialize, nil
}

type NullableProjectGroupInfo struct {
	value *ProjectGroupInfo
	isSet bool
}

func (v NullableProjectGroupInfo) Get() *ProjectGroupInfo {
	return v.value
}

func (v *NullableProjectGroupInfo) Set(val *ProjectGroupInfo) {
	v.value = val
	v.isSet = true
}

func (v NullableProjectGroupInfo) IsSet() bool {
	return v.isSet
}

func (v *NullableProjectGroupInfo) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableProjectGroupInfo(val *ProjectGroupInfo) *NullableProjectGroupInfo {
	return &NullableProjectGroupInfo{value: val, isSet: true}
}

func (v NullableProjectGroupInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableProjectGroupInfo) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
