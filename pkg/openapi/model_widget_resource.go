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

// checks if the WidgetResource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &WidgetResource{}

// WidgetResource struct for WidgetResource
type WidgetResource struct {
	Description       *string                `json:"description,omitempty"`
	Owner             *string                `json:"owner,omitempty"`
	Id                int64                  `json:"id"`
	Name              string                 `json:"name"`
	WidgetType        string                 `json:"widgetType"`
	ContentParameters ContentParameters      `json:"contentParameters"`
	AppliedFilters    []UserFilterResource   `json:"appliedFilters,omitempty"`
	Content           map[string]interface{} `json:"content,omitempty"`
}

type _WidgetResource WidgetResource

// NewWidgetResource instantiates a new WidgetResource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewWidgetResource(id int64, name string, widgetType string, contentParameters ContentParameters) *WidgetResource {
	this := WidgetResource{}
	this.Id = id
	this.Name = name
	this.WidgetType = widgetType
	this.ContentParameters = contentParameters
	return &this
}

// NewWidgetResourceWithDefaults instantiates a new WidgetResource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewWidgetResourceWithDefaults() *WidgetResource {
	this := WidgetResource{}
	return &this
}

// GetDescription returns the Description field value if set, zero value otherwise.
func (o *WidgetResource) GetDescription() string {
	if o == nil || IsNil(o.Description) {
		var ret string
		return ret
	}
	return *o.Description
}

// GetDescriptionOk returns a tuple with the Description field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetDescriptionOk() (*string, bool) {
	if o == nil || IsNil(o.Description) {
		return nil, false
	}
	return o.Description, true
}

// HasDescription returns a boolean if a field has been set.
func (o *WidgetResource) HasDescription() bool {
	if o != nil && !IsNil(o.Description) {
		return true
	}

	return false
}

// SetDescription gets a reference to the given string and assigns it to the Description field.
func (o *WidgetResource) SetDescription(v string) {
	o.Description = &v
}

// GetOwner returns the Owner field value if set, zero value otherwise.
func (o *WidgetResource) GetOwner() string {
	if o == nil || IsNil(o.Owner) {
		var ret string
		return ret
	}
	return *o.Owner
}

// GetOwnerOk returns a tuple with the Owner field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetOwnerOk() (*string, bool) {
	if o == nil || IsNil(o.Owner) {
		return nil, false
	}
	return o.Owner, true
}

// HasOwner returns a boolean if a field has been set.
func (o *WidgetResource) HasOwner() bool {
	if o != nil && !IsNil(o.Owner) {
		return true
	}

	return false
}

// SetOwner gets a reference to the given string and assigns it to the Owner field.
func (o *WidgetResource) SetOwner(v string) {
	o.Owner = &v
}

// GetId returns the Id field value
func (o *WidgetResource) GetId() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetIdOk() (*int64, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *WidgetResource) SetId(v int64) {
	o.Id = v
}

// GetName returns the Name field value
func (o *WidgetResource) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *WidgetResource) SetName(v string) {
	o.Name = v
}

// GetWidgetType returns the WidgetType field value
func (o *WidgetResource) GetWidgetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.WidgetType
}

// GetWidgetTypeOk returns a tuple with the WidgetType field value
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetWidgetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.WidgetType, true
}

// SetWidgetType sets field value
func (o *WidgetResource) SetWidgetType(v string) {
	o.WidgetType = v
}

// GetContentParameters returns the ContentParameters field value
func (o *WidgetResource) GetContentParameters() ContentParameters {
	if o == nil {
		var ret ContentParameters
		return ret
	}

	return o.ContentParameters
}

// GetContentParametersOk returns a tuple with the ContentParameters field value
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetContentParametersOk() (*ContentParameters, bool) {
	if o == nil {
		return nil, false
	}
	return &o.ContentParameters, true
}

// SetContentParameters sets field value
func (o *WidgetResource) SetContentParameters(v ContentParameters) {
	o.ContentParameters = v
}

// GetAppliedFilters returns the AppliedFilters field value if set, zero value otherwise.
func (o *WidgetResource) GetAppliedFilters() []UserFilterResource {
	if o == nil || IsNil(o.AppliedFilters) {
		var ret []UserFilterResource
		return ret
	}
	return o.AppliedFilters
}

// GetAppliedFiltersOk returns a tuple with the AppliedFilters field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetAppliedFiltersOk() ([]UserFilterResource, bool) {
	if o == nil || IsNil(o.AppliedFilters) {
		return nil, false
	}
	return o.AppliedFilters, true
}

// HasAppliedFilters returns a boolean if a field has been set.
func (o *WidgetResource) HasAppliedFilters() bool {
	if o != nil && !IsNil(o.AppliedFilters) {
		return true
	}

	return false
}

// SetAppliedFilters gets a reference to the given []UserFilterResource and assigns it to the AppliedFilters field.
func (o *WidgetResource) SetAppliedFilters(v []UserFilterResource) {
	o.AppliedFilters = v
}

// GetContent returns the Content field value if set, zero value otherwise.
func (o *WidgetResource) GetContent() map[string]interface{} {
	if o == nil || IsNil(o.Content) {
		var ret map[string]interface{}
		return ret
	}
	return o.Content
}

// GetContentOk returns a tuple with the Content field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *WidgetResource) GetContentOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Content) {
		return map[string]interface{}{}, false
	}
	return o.Content, true
}

// HasContent returns a boolean if a field has been set.
func (o *WidgetResource) HasContent() bool {
	if o != nil && !IsNil(o.Content) {
		return true
	}

	return false
}

// SetContent gets a reference to the given map[string]interface{} and assigns it to the Content field.
func (o *WidgetResource) SetContent(v map[string]interface{}) {
	o.Content = v
}

func (o WidgetResource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o WidgetResource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Description) {
		toSerialize["description"] = o.Description
	}
	if !IsNil(o.Owner) {
		toSerialize["owner"] = o.Owner
	}
	toSerialize["id"] = o.Id
	toSerialize["name"] = o.Name
	toSerialize["widgetType"] = o.WidgetType
	toSerialize["contentParameters"] = o.ContentParameters
	if !IsNil(o.AppliedFilters) {
		toSerialize["appliedFilters"] = o.AppliedFilters
	}
	if !IsNil(o.Content) {
		toSerialize["content"] = o.Content
	}
	return toSerialize, nil
}

func (o *WidgetResource) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"id",
		"name",
		"widgetType",
		"contentParameters",
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

	varWidgetResource := _WidgetResource{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varWidgetResource)

	if err != nil {
		return err
	}

	*o = WidgetResource(varWidgetResource)

	return err
}

type NullableWidgetResource struct {
	value *WidgetResource
	isSet bool
}

func (v NullableWidgetResource) Get() *WidgetResource {
	return v.value
}

func (v *NullableWidgetResource) Set(val *WidgetResource) {
	v.value = val
	v.isSet = true
}

func (v NullableWidgetResource) IsSet() bool {
	return v.isSet
}

func (v *NullableWidgetResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableWidgetResource(val *WidgetResource) *NullableWidgetResource {
	return &NullableWidgetResource{value: val, isSet: true}
}

func (v NullableWidgetResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableWidgetResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
