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
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BugTrackingSystemAPIService BugTrackingSystemAPI service
type BugTrackingSystemAPIService service

type ApiCreateIssueRequest struct {
	ctx           context.Context
	ApiService    *BugTrackingSystemAPIService
	integrationId int64
	projectName   string
	postTicketRQ  *PostTicketRQ
}

func (r ApiCreateIssueRequest) PostTicketRQ(postTicketRQ PostTicketRQ) ApiCreateIssueRequest {
	r.postTicketRQ = &postTicketRQ
	return r
}

func (r ApiCreateIssueRequest) Execute() (*Ticket, *http.Response, error) {
	return r.ApiService.CreateIssueExecute(r)
}

/*
CreateIssue Post ticket to the bts integration

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param integrationId
	@param projectName
	@return ApiCreateIssueRequest
*/
func (a *BugTrackingSystemAPIService) CreateIssue(ctx context.Context, integrationId int64, projectName string) ApiCreateIssueRequest {
	return ApiCreateIssueRequest{
		ApiService:    a,
		ctx:           ctx,
		integrationId: integrationId,
		projectName:   projectName,
	}
}

// Execute executes the request
//
//	@return Ticket
func (a *BugTrackingSystemAPIService) CreateIssueExecute(r ApiCreateIssueRequest) (*Ticket, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *Ticket
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.CreateIssue")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{projectName}/{integrationId}/ticket"
	localVarPath = strings.Replace(localVarPath, "{"+"integrationId"+"}", url.PathEscape(parameterValueToString(r.integrationId, "integrationId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.postTicketRQ == nil {
		return localVarReturnValue, nil, reportError("postTicketRQ is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	// body params
	localVarPostBody = r.postTicketRQ
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetAllowableIssueTypesRequest struct {
	ctx           context.Context
	ApiService    *BugTrackingSystemAPIService
	integrationId int64
	projectName   string
}

func (r ApiGetAllowableIssueTypesRequest) Execute() ([]string, *http.Response, error) {
	return r.ApiService.GetAllowableIssueTypesExecute(r)
}

/*
GetAllowableIssueTypes Get list of allowable issue types for bug tracking system

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param integrationId
	@param projectName
	@return ApiGetAllowableIssueTypesRequest
*/
func (a *BugTrackingSystemAPIService) GetAllowableIssueTypes(ctx context.Context, integrationId int64, projectName string) ApiGetAllowableIssueTypesRequest {
	return ApiGetAllowableIssueTypesRequest{
		ApiService:    a,
		ctx:           ctx,
		integrationId: integrationId,
		projectName:   projectName,
	}
}

// Execute executes the request
//
//	@return []string
func (a *BugTrackingSystemAPIService) GetAllowableIssueTypesExecute(r ApiGetAllowableIssueTypesRequest) ([]string, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []string
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.GetAllowableIssueTypes")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{projectName}/{integrationId}/issue_types"
	localVarPath = strings.Replace(localVarPath, "{"+"integrationId"+"}", url.PathEscape(parameterValueToString(r.integrationId, "integrationId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetAllowableIssueTypes1Request struct {
	ctx           context.Context
	ApiService    *BugTrackingSystemAPIService
	integrationId int64
}

func (r ApiGetAllowableIssueTypes1Request) Execute() ([]string, *http.Response, error) {
	return r.ApiService.GetAllowableIssueTypes1Execute(r)
}

/*
GetAllowableIssueTypes1 Get list of existed issue types in bts

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param integrationId
	@return ApiGetAllowableIssueTypes1Request
*/
func (a *BugTrackingSystemAPIService) GetAllowableIssueTypes1(ctx context.Context, integrationId int64) ApiGetAllowableIssueTypes1Request {
	return ApiGetAllowableIssueTypes1Request{
		ApiService:    a,
		ctx:           ctx,
		integrationId: integrationId,
	}
}

// Execute executes the request
//
//	@return []string
func (a *BugTrackingSystemAPIService) GetAllowableIssueTypes1Execute(r ApiGetAllowableIssueTypes1Request) ([]string, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []string
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.GetAllowableIssueTypes1")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{integrationId}/issue_types"
	localVarPath = strings.Replace(localVarPath, "{"+"integrationId"+"}", url.PathEscape(parameterValueToString(r.integrationId, "integrationId")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetSetOfIntegrationSystemFieldsRequest struct {
	ctx           context.Context
	ApiService    *BugTrackingSystemAPIService
	integrationId int64
	issueType     *string
	projectName   string
}

func (r ApiGetSetOfIntegrationSystemFieldsRequest) IssueType(issueType string) ApiGetSetOfIntegrationSystemFieldsRequest {
	r.issueType = &issueType
	return r
}

func (r ApiGetSetOfIntegrationSystemFieldsRequest) Execute() ([]PostFormField, *http.Response, error) {
	return r.ApiService.GetSetOfIntegrationSystemFieldsExecute(r)
}

/*
GetSetOfIntegrationSystemFields Get list of fields required for posting ticket in concrete integration

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param integrationId
	@param projectName
	@return ApiGetSetOfIntegrationSystemFieldsRequest
*/
func (a *BugTrackingSystemAPIService) GetSetOfIntegrationSystemFields(ctx context.Context, integrationId int64, projectName string) ApiGetSetOfIntegrationSystemFieldsRequest {
	return ApiGetSetOfIntegrationSystemFieldsRequest{
		ApiService:    a,
		ctx:           ctx,
		integrationId: integrationId,
		projectName:   projectName,
	}
}

// Execute executes the request
//
//	@return []PostFormField
func (a *BugTrackingSystemAPIService) GetSetOfIntegrationSystemFieldsExecute(r ApiGetSetOfIntegrationSystemFieldsRequest) ([]PostFormField, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []PostFormField
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.GetSetOfIntegrationSystemFields")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{projectName}/{integrationId}/fields-set"
	localVarPath = strings.Replace(localVarPath, "{"+"integrationId"+"}", url.PathEscape(parameterValueToString(r.integrationId, "integrationId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.issueType == nil {
		return localVarReturnValue, nil, reportError("issueType is required and must be specified")
	}

	parameterAddToHeaderOrQuery(localVarQueryParams, "issueType", r.issueType, "form", "")
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetSetOfIntegrationSystemFields1Request struct {
	ctx           context.Context
	ApiService    *BugTrackingSystemAPIService
	integrationId int64
	issueType     *string
}

func (r ApiGetSetOfIntegrationSystemFields1Request) IssueType(issueType string) ApiGetSetOfIntegrationSystemFields1Request {
	r.issueType = &issueType
	return r
}

func (r ApiGetSetOfIntegrationSystemFields1Request) Execute() ([]PostFormField, *http.Response, error) {
	return r.ApiService.GetSetOfIntegrationSystemFields1Execute(r)
}

/*
GetSetOfIntegrationSystemFields1 Get list of fields required for posting ticket

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param integrationId
	@return ApiGetSetOfIntegrationSystemFields1Request
*/
func (a *BugTrackingSystemAPIService) GetSetOfIntegrationSystemFields1(ctx context.Context, integrationId int64) ApiGetSetOfIntegrationSystemFields1Request {
	return ApiGetSetOfIntegrationSystemFields1Request{
		ApiService:    a,
		ctx:           ctx,
		integrationId: integrationId,
	}
}

// Execute executes the request
//
//	@return []PostFormField
func (a *BugTrackingSystemAPIService) GetSetOfIntegrationSystemFields1Execute(r ApiGetSetOfIntegrationSystemFields1Request) ([]PostFormField, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []PostFormField
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.GetSetOfIntegrationSystemFields1")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{integrationId}/fields-set"
	localVarPath = strings.Replace(localVarPath, "{"+"integrationId"+"}", url.PathEscape(parameterValueToString(r.integrationId, "integrationId")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.issueType == nil {
		return localVarReturnValue, nil, reportError("issueType is required and must be specified")
	}

	parameterAddToHeaderOrQuery(localVarQueryParams, "issueType", r.issueType, "form", "")
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetTicketRequest struct {
	ctx         context.Context
	ApiService  *BugTrackingSystemAPIService
	btsProject  *string
	btsUrl      *string
	projectName string
	ticketId    string
}

func (r ApiGetTicketRequest) BtsProject(btsProject string) ApiGetTicketRequest {
	r.btsProject = &btsProject
	return r
}

func (r ApiGetTicketRequest) BtsUrl(btsUrl string) ApiGetTicketRequest {
	r.btsUrl = &btsUrl
	return r
}

func (r ApiGetTicketRequest) Execute() (*Ticket, *http.Response, error) {
	return r.ApiService.GetTicketExecute(r)
}

/*
GetTicket Get ticket from the bts integration

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param projectName
	@param ticketId
	@return ApiGetTicketRequest
*/
func (a *BugTrackingSystemAPIService) GetTicket(ctx context.Context, projectName, ticketId string) ApiGetTicketRequest {
	return ApiGetTicketRequest{
		ApiService:  a,
		ctx:         ctx,
		projectName: projectName,
		ticketId:    ticketId,
	}
}

// Execute executes the request
//
//	@return Ticket
func (a *BugTrackingSystemAPIService) GetTicketExecute(r ApiGetTicketRequest) (*Ticket, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *Ticket
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "BugTrackingSystemAPIService.GetTicket")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/bts/{projectName}/ticket/{ticketId}"
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"ticketId"+"}", url.PathEscape(parameterValueToString(r.ticketId, "ticketId")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.btsProject == nil {
		return localVarReturnValue, nil, reportError("btsProject is required and must be specified")
	}
	if r.btsUrl == nil {
		return localVarReturnValue, nil, reportError("btsUrl is required and must be specified")
	}

	parameterAddToHeaderOrQuery(localVarQueryParams, "btsProject", r.btsProject, "form", "")
	parameterAddToHeaderOrQuery(localVarQueryParams, "btsUrl", r.btsUrl, "form", "")
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}
