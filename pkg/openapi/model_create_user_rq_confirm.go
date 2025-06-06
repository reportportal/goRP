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

// checks if the CreateUserRQConfirm type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &CreateUserRQConfirm{}

// CreateUserRQConfirm struct for CreateUserRQConfirm
type CreateUserRQConfirm struct {
	Login    string `json:"login" validate:"regexp=[a-zA-Z0-9-_.]+"`
	Password string `json:"password" validate:"regexp=^(?=.*\\\\d)(?=.*[A-Z])(?=.*[a-z])(?=.*[^a-zA-Z\\\\d\\\\s])([^\\\\s]){8,256}$"`
	FullName string `json:"fullName" validate:"regexp=[\\\\pL0-9-_ \\\\.]+"`
	Email    string `json:"email"`
}

type _CreateUserRQConfirm CreateUserRQConfirm

// NewCreateUserRQConfirm instantiates a new CreateUserRQConfirm object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewCreateUserRQConfirm(login string, password string, fullName string, email string) *CreateUserRQConfirm {
	this := CreateUserRQConfirm{}
	this.Login = login
	this.Password = password
	this.FullName = fullName
	this.Email = email
	return &this
}

// NewCreateUserRQConfirmWithDefaults instantiates a new CreateUserRQConfirm object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewCreateUserRQConfirmWithDefaults() *CreateUserRQConfirm {
	this := CreateUserRQConfirm{}
	return &this
}

// GetLogin returns the Login field value
func (o *CreateUserRQConfirm) GetLogin() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Login
}

// GetLoginOk returns a tuple with the Login field value
// and a boolean to check if the value has been set.
func (o *CreateUserRQConfirm) GetLoginOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Login, true
}

// SetLogin sets field value
func (o *CreateUserRQConfirm) SetLogin(v string) {
	o.Login = v
}

// GetPassword returns the Password field value
func (o *CreateUserRQConfirm) GetPassword() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Password
}

// GetPasswordOk returns a tuple with the Password field value
// and a boolean to check if the value has been set.
func (o *CreateUserRQConfirm) GetPasswordOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Password, true
}

// SetPassword sets field value
func (o *CreateUserRQConfirm) SetPassword(v string) {
	o.Password = v
}

// GetFullName returns the FullName field value
func (o *CreateUserRQConfirm) GetFullName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.FullName
}

// GetFullNameOk returns a tuple with the FullName field value
// and a boolean to check if the value has been set.
func (o *CreateUserRQConfirm) GetFullNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.FullName, true
}

// SetFullName sets field value
func (o *CreateUserRQConfirm) SetFullName(v string) {
	o.FullName = v
}

// GetEmail returns the Email field value
func (o *CreateUserRQConfirm) GetEmail() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Email
}

// GetEmailOk returns a tuple with the Email field value
// and a boolean to check if the value has been set.
func (o *CreateUserRQConfirm) GetEmailOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Email, true
}

// SetEmail sets field value
func (o *CreateUserRQConfirm) SetEmail(v string) {
	o.Email = v
}

func (o CreateUserRQConfirm) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o CreateUserRQConfirm) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["login"] = o.Login
	toSerialize["password"] = o.Password
	toSerialize["fullName"] = o.FullName
	toSerialize["email"] = o.Email
	return toSerialize, nil
}

func (o *CreateUserRQConfirm) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"login",
		"password",
		"fullName",
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

	varCreateUserRQConfirm := _CreateUserRQConfirm{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varCreateUserRQConfirm)

	if err != nil {
		return err
	}

	*o = CreateUserRQConfirm(varCreateUserRQConfirm)

	return err
}

type NullableCreateUserRQConfirm struct {
	value *CreateUserRQConfirm
	isSet bool
}

func (v NullableCreateUserRQConfirm) Get() *CreateUserRQConfirm {
	return v.value
}

func (v *NullableCreateUserRQConfirm) Set(val *CreateUserRQConfirm) {
	v.value = val
	v.isSet = true
}

func (v NullableCreateUserRQConfirm) IsSet() bool {
	return v.isSet
}

func (v *NullableCreateUserRQConfirm) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableCreateUserRQConfirm(val *CreateUserRQConfirm) *NullableCreateUserRQConfirm {
	return &NullableCreateUserRQConfirm{value: val, isSet: true}
}

func (v NullableCreateUserRQConfirm) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableCreateUserRQConfirm) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
