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

// checks if the CreateUserRS type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &CreateUserRS{}

// CreateUserRS struct for CreateUserRS
type CreateUserRS struct {
	Warning     *string `json:"warning,omitempty"`
	Id          *int64  `json:"id,omitempty"`
	Uuid        *string `json:"uuid,omitempty"`
	Login       *string `json:"login,omitempty"`
	Email       *string `json:"email,omitempty"`
	FullName    *string `json:"fullName,omitempty"`
	AccountRole *string `json:"accountRole,omitempty"`
	AccountType *string `json:"accountType,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	ExternalId  *string `json:"externalId,omitempty"`
}

// NewCreateUserRS instantiates a new CreateUserRS object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewCreateUserRS() *CreateUserRS {
	this := CreateUserRS{}
	return &this
}

// NewCreateUserRSWithDefaults instantiates a new CreateUserRS object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewCreateUserRSWithDefaults() *CreateUserRS {
	this := CreateUserRS{}
	return &this
}

// GetWarning returns the Warning field value if set, zero value otherwise.
func (o *CreateUserRS) GetWarning() string {
	if o == nil || IsNil(o.Warning) {
		var ret string
		return ret
	}
	return *o.Warning
}

// GetWarningOk returns a tuple with the Warning field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetWarningOk() (*string, bool) {
	if o == nil || IsNil(o.Warning) {
		return nil, false
	}
	return o.Warning, true
}

// HasWarning returns a boolean if a field has been set.
func (o *CreateUserRS) HasWarning() bool {
	if o != nil && !IsNil(o.Warning) {
		return true
	}

	return false
}

// SetWarning gets a reference to the given string and assigns it to the Warning field.
func (o *CreateUserRS) SetWarning(v string) {
	o.Warning = &v
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *CreateUserRS) GetId() int64 {
	if o == nil || IsNil(o.Id) {
		var ret int64
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetIdOk() (*int64, bool) {
	if o == nil || IsNil(o.Id) {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *CreateUserRS) HasId() bool {
	if o != nil && !IsNil(o.Id) {
		return true
	}

	return false
}

// SetId gets a reference to the given int64 and assigns it to the Id field.
func (o *CreateUserRS) SetId(v int64) {
	o.Id = &v
}

// GetUuid returns the Uuid field value if set, zero value otherwise.
func (o *CreateUserRS) GetUuid() string {
	if o == nil || IsNil(o.Uuid) {
		var ret string
		return ret
	}
	return *o.Uuid
}

// GetUuidOk returns a tuple with the Uuid field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetUuidOk() (*string, bool) {
	if o == nil || IsNil(o.Uuid) {
		return nil, false
	}
	return o.Uuid, true
}

// HasUuid returns a boolean if a field has been set.
func (o *CreateUserRS) HasUuid() bool {
	if o != nil && !IsNil(o.Uuid) {
		return true
	}

	return false
}

// SetUuid gets a reference to the given string and assigns it to the Uuid field.
func (o *CreateUserRS) SetUuid(v string) {
	o.Uuid = &v
}

// GetLogin returns the Login field value if set, zero value otherwise.
func (o *CreateUserRS) GetLogin() string {
	if o == nil || IsNil(o.Login) {
		var ret string
		return ret
	}
	return *o.Login
}

// GetLoginOk returns a tuple with the Login field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetLoginOk() (*string, bool) {
	if o == nil || IsNil(o.Login) {
		return nil, false
	}
	return o.Login, true
}

// HasLogin returns a boolean if a field has been set.
func (o *CreateUserRS) HasLogin() bool {
	if o != nil && !IsNil(o.Login) {
		return true
	}

	return false
}

// SetLogin gets a reference to the given string and assigns it to the Login field.
func (o *CreateUserRS) SetLogin(v string) {
	o.Login = &v
}

// GetEmail returns the Email field value if set, zero value otherwise.
func (o *CreateUserRS) GetEmail() string {
	if o == nil || IsNil(o.Email) {
		var ret string
		return ret
	}
	return *o.Email
}

// GetEmailOk returns a tuple with the Email field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetEmailOk() (*string, bool) {
	if o == nil || IsNil(o.Email) {
		return nil, false
	}
	return o.Email, true
}

// HasEmail returns a boolean if a field has been set.
func (o *CreateUserRS) HasEmail() bool {
	if o != nil && !IsNil(o.Email) {
		return true
	}

	return false
}

// SetEmail gets a reference to the given string and assigns it to the Email field.
func (o *CreateUserRS) SetEmail(v string) {
	o.Email = &v
}

// GetFullName returns the FullName field value if set, zero value otherwise.
func (o *CreateUserRS) GetFullName() string {
	if o == nil || IsNil(o.FullName) {
		var ret string
		return ret
	}
	return *o.FullName
}

// GetFullNameOk returns a tuple with the FullName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetFullNameOk() (*string, bool) {
	if o == nil || IsNil(o.FullName) {
		return nil, false
	}
	return o.FullName, true
}

// HasFullName returns a boolean if a field has been set.
func (o *CreateUserRS) HasFullName() bool {
	if o != nil && !IsNil(o.FullName) {
		return true
	}

	return false
}

// SetFullName gets a reference to the given string and assigns it to the FullName field.
func (o *CreateUserRS) SetFullName(v string) {
	o.FullName = &v
}

// GetAccountRole returns the AccountRole field value if set, zero value otherwise.
func (o *CreateUserRS) GetAccountRole() string {
	if o == nil || IsNil(o.AccountRole) {
		var ret string
		return ret
	}
	return *o.AccountRole
}

// GetAccountRoleOk returns a tuple with the AccountRole field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetAccountRoleOk() (*string, bool) {
	if o == nil || IsNil(o.AccountRole) {
		return nil, false
	}
	return o.AccountRole, true
}

// HasAccountRole returns a boolean if a field has been set.
func (o *CreateUserRS) HasAccountRole() bool {
	if o != nil && !IsNil(o.AccountRole) {
		return true
	}

	return false
}

// SetAccountRole gets a reference to the given string and assigns it to the AccountRole field.
func (o *CreateUserRS) SetAccountRole(v string) {
	o.AccountRole = &v
}

// GetAccountType returns the AccountType field value if set, zero value otherwise.
func (o *CreateUserRS) GetAccountType() string {
	if o == nil || IsNil(o.AccountType) {
		var ret string
		return ret
	}
	return *o.AccountType
}

// GetAccountTypeOk returns a tuple with the AccountType field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetAccountTypeOk() (*string, bool) {
	if o == nil || IsNil(o.AccountType) {
		return nil, false
	}
	return o.AccountType, true
}

// HasAccountType returns a boolean if a field has been set.
func (o *CreateUserRS) HasAccountType() bool {
	if o != nil && !IsNil(o.AccountType) {
		return true
	}

	return false
}

// SetAccountType gets a reference to the given string and assigns it to the AccountType field.
func (o *CreateUserRS) SetAccountType(v string) {
	o.AccountType = &v
}

// GetActive returns the Active field value if set, zero value otherwise.
func (o *CreateUserRS) GetActive() bool {
	if o == nil || IsNil(o.Active) {
		var ret bool
		return ret
	}
	return *o.Active
}

// GetActiveOk returns a tuple with the Active field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetActiveOk() (*bool, bool) {
	if o == nil || IsNil(o.Active) {
		return nil, false
	}
	return o.Active, true
}

// HasActive returns a boolean if a field has been set.
func (o *CreateUserRS) HasActive() bool {
	if o != nil && !IsNil(o.Active) {
		return true
	}

	return false
}

// SetActive gets a reference to the given bool and assigns it to the Active field.
func (o *CreateUserRS) SetActive(v bool) {
	o.Active = &v
}

// GetExternalId returns the ExternalId field value if set, zero value otherwise.
func (o *CreateUserRS) GetExternalId() string {
	if o == nil || IsNil(o.ExternalId) {
		var ret string
		return ret
	}
	return *o.ExternalId
}

// GetExternalIdOk returns a tuple with the ExternalId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *CreateUserRS) GetExternalIdOk() (*string, bool) {
	if o == nil || IsNil(o.ExternalId) {
		return nil, false
	}
	return o.ExternalId, true
}

// HasExternalId returns a boolean if a field has been set.
func (o *CreateUserRS) HasExternalId() bool {
	if o != nil && !IsNil(o.ExternalId) {
		return true
	}

	return false
}

// SetExternalId gets a reference to the given string and assigns it to the ExternalId field.
func (o *CreateUserRS) SetExternalId(v string) {
	o.ExternalId = &v
}

func (o CreateUserRS) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o CreateUserRS) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Warning) {
		toSerialize["warning"] = o.Warning
	}
	if !IsNil(o.Id) {
		toSerialize["id"] = o.Id
	}
	if !IsNil(o.Uuid) {
		toSerialize["uuid"] = o.Uuid
	}
	if !IsNil(o.Login) {
		toSerialize["login"] = o.Login
	}
	if !IsNil(o.Email) {
		toSerialize["email"] = o.Email
	}
	if !IsNil(o.FullName) {
		toSerialize["fullName"] = o.FullName
	}
	if !IsNil(o.AccountRole) {
		toSerialize["accountRole"] = o.AccountRole
	}
	if !IsNil(o.AccountType) {
		toSerialize["accountType"] = o.AccountType
	}
	if !IsNil(o.Active) {
		toSerialize["active"] = o.Active
	}
	if !IsNil(o.ExternalId) {
		toSerialize["externalId"] = o.ExternalId
	}
	return toSerialize, nil
}

type NullableCreateUserRS struct {
	value *CreateUserRS
	isSet bool
}

func (v NullableCreateUserRS) Get() *CreateUserRS {
	return v.value
}

func (v *NullableCreateUserRS) Set(val *CreateUserRS) {
	v.value = val
	v.isSet = true
}

func (v NullableCreateUserRS) IsSet() bool {
	return v.isSet
}

func (v *NullableCreateUserRS) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableCreateUserRS(val *CreateUserRS) *NullableCreateUserRS {
	return &NullableCreateUserRS{value: val, isSet: true}
}

func (v NullableCreateUserRS) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableCreateUserRS) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
