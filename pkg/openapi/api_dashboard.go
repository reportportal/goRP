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

// DashboardAPIService DashboardAPI service
type DashboardAPIService service

type ApiAddWidgetRequest struct {
	ctx         context.Context
	ApiService  *DashboardAPIService
	dashboardId int64
	projectName string
	addWidgetRq *AddWidgetRq
}

func (r ApiAddWidgetRequest) AddWidgetRq(addWidgetRq AddWidgetRq) ApiAddWidgetRequest {
	r.addWidgetRq = &addWidgetRq
	return r
}

func (r ApiAddWidgetRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.AddWidgetExecute(r)
}

/*
AddWidget Add widget to specified dashboard

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@return ApiAddWidgetRequest
*/
func (a *DashboardAPIService) AddWidget(ctx context.Context, dashboardId int64, projectName string) ApiAddWidgetRequest {
	return ApiAddWidgetRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *DashboardAPIService) AddWidgetExecute(r ApiAddWidgetRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPut
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.AddWidget")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}/add"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.addWidgetRq == nil {
		return localVarReturnValue, nil, reportError("addWidgetRq is required and must be specified")
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
	localVarPostBody = r.addWidgetRq
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiCreateDashboardRequest struct {
	ctx               context.Context
	ApiService        *DashboardAPIService
	projectName       string
	createDashboardRQ *CreateDashboardRQ
}

func (r ApiCreateDashboardRequest) CreateDashboardRQ(createDashboardRQ CreateDashboardRQ) ApiCreateDashboardRequest {
	r.createDashboardRQ = &createDashboardRQ
	return r
}

func (r ApiCreateDashboardRequest) Execute() (*EntryCreatedRS, *http.Response, error) {
	return r.ApiService.CreateDashboardExecute(r)
}

/*
CreateDashboard Create dashboard for specified project

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param projectName
	@return ApiCreateDashboardRequest
*/
func (a *DashboardAPIService) CreateDashboard(ctx context.Context, projectName string) ApiCreateDashboardRequest {
	return ApiCreateDashboardRequest{
		ApiService:  a,
		ctx:         ctx,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return EntryCreatedRS
func (a *DashboardAPIService) CreateDashboardExecute(r ApiCreateDashboardRequest) (*EntryCreatedRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *EntryCreatedRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.CreateDashboard")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard"
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.createDashboardRQ == nil {
		return localVarReturnValue, nil, reportError("createDashboardRQ is required and must be specified")
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
	localVarPostBody = r.createDashboardRQ
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiCreatePreconfiguredRequest struct {
	ctx                      context.Context
	ApiService               *DashboardAPIService
	projectName              string
	dashboardPreconfiguredRq *DashboardPreconfiguredRq
}

func (r ApiCreatePreconfiguredRequest) DashboardPreconfiguredRq(dashboardPreconfiguredRq DashboardPreconfiguredRq) ApiCreatePreconfiguredRequest {
	r.dashboardPreconfiguredRq = &dashboardPreconfiguredRq
	return r
}

func (r ApiCreatePreconfiguredRequest) Execute() (*EntryCreatedRS, *http.Response, error) {
	return r.ApiService.CreatePreconfiguredExecute(r)
}

/*
CreatePreconfigured Create Dashboard with provided configuration including its widgets and filters if any

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param projectName
	@return ApiCreatePreconfiguredRequest
*/
func (a *DashboardAPIService) CreatePreconfigured(ctx context.Context, projectName string) ApiCreatePreconfiguredRequest {
	return ApiCreatePreconfiguredRequest{
		ApiService:  a,
		ctx:         ctx,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return EntryCreatedRS
func (a *DashboardAPIService) CreatePreconfiguredExecute(r ApiCreatePreconfiguredRequest) (*EntryCreatedRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *EntryCreatedRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.CreatePreconfigured")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/preconfigured"
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.dashboardPreconfiguredRq == nil {
		return localVarReturnValue, nil, reportError("dashboardPreconfiguredRq is required and must be specified")
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
	localVarPostBody = r.dashboardPreconfiguredRq
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiDeleteDashboardRequest struct {
	ctx         context.Context
	ApiService  *DashboardAPIService
	dashboardId int64
	projectName string
}

func (r ApiDeleteDashboardRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.DeleteDashboardExecute(r)
}

/*
DeleteDashboard Delete specified dashboard by ID for specified project

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@return ApiDeleteDashboardRequest
*/
func (a *DashboardAPIService) DeleteDashboard(ctx context.Context, dashboardId int64, projectName string) ApiDeleteDashboardRequest {
	return ApiDeleteDashboardRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *DashboardAPIService) DeleteDashboardExecute(r ApiDeleteDashboardRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodDelete
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.DeleteDashboard")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiGetAllDashboardsRequest struct {
	ctx                  context.Context
	ApiService           *DashboardAPIService
	projectName          string
	pagePage             *int32
	pageSize             *int32
	pageSort             *string
	filterEqCreationDate *string
	filterEqOwner        *string
	filterEqId           *int32
	filterEqName         *string
	filterEqProjectId    *int32
}

// Results page you want to retrieve (0..N)
func (r ApiGetAllDashboardsRequest) PagePage(pagePage int32) ApiGetAllDashboardsRequest {
	r.pagePage = &pagePage
	return r
}

// Number of records per page
func (r ApiGetAllDashboardsRequest) PageSize(pageSize int32) ApiGetAllDashboardsRequest {
	r.pageSize = &pageSize
	return r
}

// Sorting criteria in the format: property, (asc|desc). Default sort order is ascending. Multiple sort criteria are supported.
func (r ApiGetAllDashboardsRequest) PageSort(pageSort string) ApiGetAllDashboardsRequest {
	r.pageSort = &pageSort
	return r
}

// Filters by &#39;creationDate&#39;
func (r ApiGetAllDashboardsRequest) FilterEqCreationDate(filterEqCreationDate string) ApiGetAllDashboardsRequest {
	r.filterEqCreationDate = &filterEqCreationDate
	return r
}

// Filters by &#39;owner&#39;
func (r ApiGetAllDashboardsRequest) FilterEqOwner(filterEqOwner string) ApiGetAllDashboardsRequest {
	r.filterEqOwner = &filterEqOwner
	return r
}

// Filters by &#39;id&#39;
func (r ApiGetAllDashboardsRequest) FilterEqId(filterEqId int32) ApiGetAllDashboardsRequest {
	r.filterEqId = &filterEqId
	return r
}

// Filters by &#39;name&#39;
func (r ApiGetAllDashboardsRequest) FilterEqName(filterEqName string) ApiGetAllDashboardsRequest {
	r.filterEqName = &filterEqName
	return r
}

// Filters by &#39;projectId&#39;
func (r ApiGetAllDashboardsRequest) FilterEqProjectId(filterEqProjectId int32) ApiGetAllDashboardsRequest {
	r.filterEqProjectId = &filterEqProjectId
	return r
}

func (r ApiGetAllDashboardsRequest) Execute() (*PageDashboardResource, *http.Response, error) {
	return r.ApiService.GetAllDashboardsExecute(r)
}

/*
GetAllDashboards Get all permitted dashboard resources for specified project

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param projectName
	@return ApiGetAllDashboardsRequest
*/
func (a *DashboardAPIService) GetAllDashboards(ctx context.Context, projectName string) ApiGetAllDashboardsRequest {
	return ApiGetAllDashboardsRequest{
		ApiService:  a,
		ctx:         ctx,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return PageDashboardResource
func (a *DashboardAPIService) GetAllDashboardsExecute(r ApiGetAllDashboardsRequest) (*PageDashboardResource, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *PageDashboardResource
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.GetAllDashboards")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard"
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	if r.pagePage != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "page.page", r.pagePage, "form", "")
	}
	if r.pageSize != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "page.size", r.pageSize, "form", "")
	}
	if r.pageSort != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "page.sort", r.pageSort, "form", "")
	}
	if r.filterEqCreationDate != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "filter.eq.creationDate", r.filterEqCreationDate, "form", "")
	}
	if r.filterEqOwner != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "filter.eq.owner", r.filterEqOwner, "form", "")
	}
	if r.filterEqId != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "filter.eq.id", r.filterEqId, "form", "")
	}
	if r.filterEqName != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "filter.eq.name", r.filterEqName, "form", "")
	}
	if r.filterEqProjectId != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "filter.eq.projectId", r.filterEqProjectId, "form", "")
	}
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiGetDashboardRequest struct {
	ctx         context.Context
	ApiService  *DashboardAPIService
	dashboardId int64
	projectName string
}

func (r ApiGetDashboardRequest) Execute() (*DashboardResource, *http.Response, error) {
	return r.ApiService.GetDashboardExecute(r)
}

/*
GetDashboard Get specified dashboard by ID for specified project

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@return ApiGetDashboardRequest
*/
func (a *DashboardAPIService) GetDashboard(ctx context.Context, dashboardId int64, projectName string) ApiGetDashboardRequest {
	return ApiGetDashboardRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return DashboardResource
func (a *DashboardAPIService) GetDashboardExecute(r ApiGetDashboardRequest) (*DashboardResource, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *DashboardResource
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.GetDashboard")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiGetDashboardConfigRequest struct {
	ctx         context.Context
	ApiService  *DashboardAPIService
	dashboardId int64
	projectName string
}

func (r ApiGetDashboardConfigRequest) Execute() (*DashboardConfigResource, *http.Response, error) {
	return r.ApiService.GetDashboardConfigExecute(r)
}

/*
GetDashboardConfig Get Dashboard configuration including its widgets and filters if any

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@return ApiGetDashboardConfigRequest
*/
func (a *DashboardAPIService) GetDashboardConfig(ctx context.Context, dashboardId int64, projectName string) ApiGetDashboardConfigRequest {
	return ApiGetDashboardConfigRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return DashboardConfigResource
func (a *DashboardAPIService) GetDashboardConfigExecute(r ApiGetDashboardConfigRequest) (*DashboardConfigResource, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *DashboardConfigResource
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.GetDashboardConfig")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}/config"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiRemoveWidgetRequest struct {
	ctx         context.Context
	ApiService  *DashboardAPIService
	dashboardId int64
	projectName string
	widgetId    int64
}

func (r ApiRemoveWidgetRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.RemoveWidgetExecute(r)
}

/*
RemoveWidget Remove widget from specified dashboard

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@param widgetId
	@return ApiRemoveWidgetRequest
*/
func (a *DashboardAPIService) RemoveWidget(ctx context.Context, dashboardId int64, projectName string, widgetId int64) ApiRemoveWidgetRequest {
	return ApiRemoveWidgetRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
		widgetId:    widgetId,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *DashboardAPIService) RemoveWidgetExecute(r ApiRemoveWidgetRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodDelete
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.RemoveWidget")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}/{widgetId}"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"widgetId"+"}", url.PathEscape(parameterValueToString(r.widgetId, "widgetId")), -1)

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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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

type ApiUpdateDashboardRequest struct {
	ctx               context.Context
	ApiService        *DashboardAPIService
	dashboardId       int64
	projectName       string
	updateDashboardRQ *UpdateDashboardRQ
}

func (r ApiUpdateDashboardRequest) UpdateDashboardRQ(updateDashboardRQ UpdateDashboardRQ) ApiUpdateDashboardRequest {
	r.updateDashboardRQ = &updateDashboardRQ
	return r
}

func (r ApiUpdateDashboardRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.UpdateDashboardExecute(r)
}

/*
UpdateDashboard Update specified dashboard for specified project

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param dashboardId
	@param projectName
	@return ApiUpdateDashboardRequest
*/
func (a *DashboardAPIService) UpdateDashboard(ctx context.Context, dashboardId int64, projectName string) ApiUpdateDashboardRequest {
	return ApiUpdateDashboardRequest{
		ApiService:  a,
		ctx:         ctx,
		dashboardId: dashboardId,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *DashboardAPIService) UpdateDashboardExecute(r ApiUpdateDashboardRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPut
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DashboardAPIService.UpdateDashboard")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/{projectName}/dashboard/{dashboardId}"
	localVarPath = strings.Replace(localVarPath, "{"+"dashboardId"+"}", url.PathEscape(parameterValueToString(r.dashboardId, "dashboardId")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.updateDashboardRQ == nil {
		return localVarReturnValue, nil, reportError("updateDashboardRQ is required and must be specified")
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
	localVarPostBody = r.updateDashboardRQ
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

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarReturnValue, localVarHTTPResponse, err
		}
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
