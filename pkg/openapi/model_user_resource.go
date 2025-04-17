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

// checks if the UserResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &UserResource{}

// UserResource struct for UserResource
type UserResource struct {
	Uuid             *string                    `json:"uuid,omitempty"`
	ExternalId       *string                    `json:"externalId,omitempty"`
	Active           *bool                      `json:"active,omitempty"`
	Id               int64                      `json:"id"`
	UserId           string                     `json:"userId"`
	Email            string                     `json:"email"`
	PhotoId          *string                    `json:"photoId,omitempty"`
	FullName         *string                    `json:"fullName,omitempty"`
	AccountType      *string                    `json:"accountType,omitempty"`
	UserRole         *string                    `json:"userRole,omitempty"`
	PhotoLoaded      *bool                      `json:"photoLoaded,omitempty"`
	Metadata         map[string]interface{}     `json:"metadata,omitempty"`
	AssignedProjects map[string]AssignedProject `json:"assignedProjects,omitempty"`
}

type _UserResource UserResource

// NewUserResource instantiates a new UserResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewUserResource(id int64, userId, email string) *UserResource {
	this := UserResource{}
	this.Id = id
	this.UserId = userId
	this.Email = email
	return &this
}

// NewUserResourceWithDefaults instantiates a new UserResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewUserResourceWithDefaults() *UserResource {
	this := UserResource{}
	return &this
}

// GetUuid returns the Uuid field value if set, zero value otherwise.
func (o *UserResource) GetUuid() string {
	if o == nil || IsNil(o.Uuid) {
		var ret string
		return ret
	}
	return *o.Uuid
}

// GetUuidOk returns a tuple with the Uuid field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetUuidOk() (*string, bool) {
	if o == nil || IsNil(o.Uuid) {
		return nil, false
	}
	return o.Uuid, true
}

// HasUuid returns a boolean if a field has been set.
func (o *UserResource) HasUuid() bool {
	if o != nil && !IsNil(o.Uuid) {
		return true
	}

	return false
}

// SetUuid gets a reference to the given string and assigns it to the Uuid field.
func (o *UserResource) SetUuid(v string) {
	o.Uuid = &v
}

// GetExternalId returns the ExternalId field value if set, zero value otherwise.
func (o *UserResource) GetExternalId() string {
	if o == nil || IsNil(o.ExternalId) {
		var ret string
		return ret
	}
	return *o.ExternalId
}

// GetExternalIdOk returns a tuple with the ExternalId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetExternalIdOk() (*string, bool) {
	if o == nil || IsNil(o.ExternalId) {
		return nil, false
	}
	return o.ExternalId, true
}

// HasExternalId returns a boolean if a field has been set.
func (o *UserResource) HasExternalId() bool {
	if o != nil && !IsNil(o.ExternalId) {
		return true
	}

	return false
}

// SetExternalId gets a reference to the given string and assigns it to the ExternalId field.
func (o *UserResource) SetExternalId(v string) {
	o.ExternalId = &v
}

// GetActive returns the Active field value if set, zero value otherwise.
func (o *UserResource) GetActive() bool {
	if o == nil || IsNil(o.Active) {
		var ret bool
		return ret
	}
	return *o.Active
}

// GetActiveOk returns a tuple with the Active field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetActiveOk() (*bool, bool) {
	if o == nil || IsNil(o.Active) {
		return nil, false
	}
	return o.Active, true
}

// HasActive returns a boolean if a field has been set.
func (o *UserResource) HasActive() bool {
	if o != nil && !IsNil(o.Active) {
		return true
	}

	return false
}

// SetActive gets a reference to the given bool and assigns it to the Active field.
func (o *UserResource) SetActive(v bool) {
	o.Active = &v
}

// GetId returns the Id field value
func (o *UserResource) GetId() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *UserResource) GetIdOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *UserResource) SetId(v int64) {
	o.Id = v
}

// GetUserId returns the UserId field value
func (o *UserResource) GetUserId() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.UserId
}

// GetUserIdOk returns a tuple with the UserId field value
// and a boolean to check if the value has been set.
func (o *UserResource) GetUserIdOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.UserId, true
}

// SetUserId sets field value
func (o *UserResource) SetUserId(v string) {
	o.UserId = v
}

// GetEmail returns the Email field value
func (o *UserResource) GetEmail() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Email
}

// GetEmailOk returns a tuple with the Email field value
// and a boolean to check if the value has been set.
func (o *UserResource) GetEmailOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Email, true
}

// SetEmail sets field value
func (o *UserResource) SetEmail(v string) {
	o.Email = v
}

// GetPhotoId returns the PhotoId field value if set, zero value otherwise.
func (o *UserResource) GetPhotoId() string {
	if o == nil || IsNil(o.PhotoId) {
		var ret string
		return ret
	}
	return *o.PhotoId
}

// GetPhotoIdOk returns a tuple with the PhotoId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetPhotoIdOk() (*string, bool) {
	if o == nil || IsNil(o.PhotoId) {
		return nil, false
	}
	return o.PhotoId, true
}

// HasPhotoId returns a boolean if a field has been set.
func (o *UserResource) HasPhotoId() bool {
	if o != nil && !IsNil(o.PhotoId) {
		return true
	}

	return false
}

// SetPhotoId gets a reference to the given string and assigns it to the PhotoId field.
func (o *UserResource) SetPhotoId(v string) {
	o.PhotoId = &v
}

// GetFullName returns the FullName field value if set, zero value otherwise.
func (o *UserResource) GetFullName() string {
	if o == nil || IsNil(o.FullName) {
		var ret string
		return ret
	}
	return *o.FullName
}

// GetFullNameOk returns a tuple with the FullName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetFullNameOk() (*string, bool) {
	if o == nil || IsNil(o.FullName) {
		return nil, false
	}
	return o.FullName, true
}

// HasFullName returns a boolean if a field has been set.
func (o *UserResource) HasFullName() bool {
	if o != nil && !IsNil(o.FullName) {
		return true
	}

	return false
}

// SetFullName gets a reference to the given string and assigns it to the FullName field.
func (o *UserResource) SetFullName(v string) {
	o.FullName = &v
}

// GetAccountType returns the AccountType field value if set, zero value otherwise.
func (o *UserResource) GetAccountType() string {
	if o == nil || IsNil(o.AccountType) {
		var ret string
		return ret
	}
	return *o.AccountType
}

// GetAccountTypeOk returns a tuple with the AccountType field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetAccountTypeOk() (*string, bool) {
	if o == nil || IsNil(o.AccountType) {
		return nil, false
	}
	return o.AccountType, true
}

// HasAccountType returns a boolean if a field has been set.
func (o *UserResource) HasAccountType() bool {
	if o != nil && !IsNil(o.AccountType) {
		return true
	}

	return false
}

// SetAccountType gets a reference to the given string and assigns it to the AccountType field.
func (o *UserResource) SetAccountType(v string) {
	o.AccountType = &v
}

// GetUserRole returns the UserRole field value if set, zero value otherwise.
func (o *UserResource) GetUserRole() string {
	if o == nil || IsNil(o.UserRole) {
		var ret string
		return ret
	}
	return *o.UserRole
}

// GetUserRoleOk returns a tuple with the UserRole field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetUserRoleOk() (*string, bool) {
	if o == nil || IsNil(o.UserRole) {
		return nil, false
	}
	return o.UserRole, true
}

// HasUserRole returns a boolean if a field has been set.
func (o *UserResource) HasUserRole() bool {
	if o != nil && !IsNil(o.UserRole) {
		return true
	}

	return false
}

// SetUserRole gets a reference to the given string and assigns it to the UserRole field.
func (o *UserResource) SetUserRole(v string) {
	o.UserRole = &v
}

// GetPhotoLoaded returns the PhotoLoaded field value if set, zero value otherwise.
func (o *UserResource) GetPhotoLoaded() bool {
	if o == nil || IsNil(o.PhotoLoaded) {
		var ret bool
		return ret
	}
	return *o.PhotoLoaded
}

// GetPhotoLoadedOk returns a tuple with the PhotoLoaded field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetPhotoLoadedOk() (*bool, bool) {
	if o == nil || IsNil(o.PhotoLoaded) {
		return nil, false
	}
	return o.PhotoLoaded, true
}

// HasPhotoLoaded returns a boolean if a field has been set.
func (o *UserResource) HasPhotoLoaded() bool {
	if o != nil && !IsNil(o.PhotoLoaded) {
		return true
	}

	return false
}

// SetPhotoLoaded gets a reference to the given bool and assigns it to the PhotoLoaded field.
func (o *UserResource) SetPhotoLoaded(v bool) {
	o.PhotoLoaded = &v
}

// GetMetadata returns the Metadata field value if set, zero value otherwise.
func (o *UserResource) GetMetadata() map[string]interface{} {
	if o == nil || IsNil(o.Metadata) {
		var ret map[string]interface{}
		return ret
	}
	return o.Metadata
}

// GetMetadataOk returns a tuple with the Metadata field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetMetadataOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Metadata) {
		return map[string]interface{}{}, false
	}
	return o.Metadata, true
}

// HasMetadata returns a boolean if a field has been set.
func (o *UserResource) HasMetadata() bool {
	if o != nil && !IsNil(o.Metadata) {
		return true
	}

	return false
}

// SetMetadata gets a reference to the given map[string]interface{} and assigns it to the Metadata field.
func (o *UserResource) SetMetadata(v map[string]interface{}) {
	o.Metadata = v
}

// GetAssignedProjects returns the AssignedProjects field value if set, zero value otherwise.
func (o *UserResource) GetAssignedProjects() map[string]AssignedProject {
	if o == nil || IsNil(o.AssignedProjects) {
		var ret map[string]AssignedProject
		return ret
	}
	return o.AssignedProjects
}

// GetAssignedProjectsOk returns a tuple with the AssignedProjects field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *UserResource) GetAssignedProjectsOk() (map[string]AssignedProject, bool) {
	if o == nil || IsNil(o.AssignedProjects) {
		return map[string]AssignedProject{}, false
	}
	return o.AssignedProjects, true
}

// HasAssignedProjects returns a boolean if a field has been set.
func (o *UserResource) HasAssignedProjects() bool {
	if o != nil && !IsNil(o.AssignedProjects) {
		return true
	}

	return false
}

// SetAssignedProjects gets a reference to the given map[string]AssignedProject and assigns it to the AssignedProjects field.
func (o *UserResource) SetAssignedProjects(v map[string]AssignedProject) {
	o.AssignedProjects = v
}

func (o UserResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o UserResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Uuid) {
		toSerialize["uuid"] = o.Uuid
	}
	if !IsNil(o.ExternalId) {
		toSerialize["externalId"] = o.ExternalId
	}
	if !IsNil(o.Active) {
		toSerialize["active"] = o.Active
	}
	toSerialize["id"] = o.Id
	toSerialize["userId"] = o.UserId
	toSerialize["email"] = o.Email
	if !IsNil(o.PhotoId) {
		toSerialize["photoId"] = o.PhotoId
	}
	if !IsNil(o.FullName) {
		toSerialize["fullName"] = o.FullName
	}
	if !IsNil(o.AccountType) {
		toSerialize["accountType"] = o.AccountType
	}
	if !IsNil(o.UserRole) {
		toSerialize["userRole"] = o.UserRole
	}
	if !IsNil(o.PhotoLoaded) {
		toSerialize["photoLoaded"] = o.PhotoLoaded
	}
	if !IsNil(o.Metadata) {
		toSerialize["metadata"] = o.Metadata
	}
	if !IsNil(o.AssignedProjects) {
		toSerialize["assignedProjects"] = o.AssignedProjects
	}
	return toSerialize, nil
}

func (o *UserResource) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"id",
		"userId",
		"email",
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

	varUserResource := _UserResource{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varUserResource)

	if err != nil {
		return err
	}

	*o = UserResource(varUserResource)

	return err
}

type NullableUserResource struct {
	value *UserResource
	isSet bool
}

func (v NullableUserResource) Get() *UserResource {
	return v.value
}

func (v *NullableUserResource) Set(val *UserResource) {
	v.value = val
	v.isSet = true
}

func (v NullableUserResource) IsSet() bool {
	return v.isSet
}

func (v *NullableUserResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableUserResource(val *UserResource) *NullableUserResource {
	return &NullableUserResource{value: val, isSet: true}
}

func (v NullableUserResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableUserResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
