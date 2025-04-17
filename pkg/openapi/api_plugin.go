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
	"os"
	"strings"
)

// PluginAPIService PluginAPI service
type PluginAPIService service

type ApiDeletePluginRequest struct {
	ctx        context.Context
	ApiService *PluginAPIService
	pluginId   int64
}

func (r ApiDeletePluginRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.DeletePluginExecute(r)
}

/*
DeletePlugin Delete plugin by id

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param pluginId
	@return ApiDeletePluginRequest
*/
func (a *PluginAPIService) DeletePlugin(ctx context.Context, pluginId int64) ApiDeletePluginRequest {
	return ApiDeletePluginRequest{
		ApiService: a,
		ctx:        ctx,
		pluginId:   pluginId,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *PluginAPIService) DeletePluginExecute(r ApiDeletePluginRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodDelete
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.DeletePlugin")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/{pluginId}"
	localVarPath = strings.Replace(localVarPath, "{"+"pluginId"+"}", url.PathEscape(parameterValueToString(r.pluginId, "pluginId")), -1)

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

type ApiExecuteImportPluginCommandRequest struct {
	ctx            context.Context
	ApiService     *PluginAPIService
	pluginName     string
	projectName    string
	file           *os.File
	launchImportRq *LaunchImportRQ
}

func (r ApiExecuteImportPluginCommandRequest) File(file *os.File) ApiExecuteImportPluginCommandRequest {
	r.file = file
	return r
}

func (r ApiExecuteImportPluginCommandRequest) LaunchImportRq(launchImportRq LaunchImportRQ) ApiExecuteImportPluginCommandRequest {
	r.launchImportRq = &launchImportRq
	return r
}

func (r ApiExecuteImportPluginCommandRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return r.ApiService.ExecuteImportPluginCommandExecute(r)
}

/*
ExecuteImportPluginCommand Send report to the specified plugin for importing

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param pluginName
	@param projectName
	@return ApiExecuteImportPluginCommandRequest
*/
func (a *PluginAPIService) ExecuteImportPluginCommand(ctx context.Context, pluginName, projectName string) ApiExecuteImportPluginCommandRequest {
	return ApiExecuteImportPluginCommandRequest{
		ApiService:  a,
		ctx:         ctx,
		pluginName:  pluginName,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return map[string]interface{}
func (a *PluginAPIService) ExecuteImportPluginCommandExecute(r ApiExecuteImportPluginCommandRequest) (map[string]interface{}, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue map[string]interface{}
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.ExecuteImportPluginCommand")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/{projectName}/{pluginName}/import"
	localVarPath = strings.Replace(localVarPath, "{"+"pluginName"+"}", url.PathEscape(parameterValueToString(r.pluginName, "pluginName")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.file == nil {
		return localVarReturnValue, nil, reportError("file is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"multipart/form-data"}

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
	var fileLocalVarFormFileName string
	var fileLocalVarFileName string
	var fileLocalVarFileBytes []byte

	fileLocalVarFormFileName = "file"
	fileLocalVarFile := r.file

	if fileLocalVarFile != nil {
		fbs, _ := io.ReadAll(fileLocalVarFile)

		fileLocalVarFileBytes = fbs
		fileLocalVarFileName = fileLocalVarFile.Name()
		fileLocalVarFile.Close()
		formFiles = append(formFiles, formFile{fileBytes: fileLocalVarFileBytes, fileName: fileLocalVarFileName, formFileName: fileLocalVarFormFileName})
	}
	if r.launchImportRq != nil {
		paramJson, err := parameterToJson(*r.launchImportRq)
		if err != nil {
			return localVarReturnValue, nil, err
		}
		localVarFormParams.Add("launchImportRq", paramJson)
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

type ApiExecutePluginCommandRequest struct {
	ctx         context.Context
	ApiService  *PluginAPIService
	command     string
	pluginName  string
	projectName string
	requestBody *map[string]map[string]interface{}
}

func (r ApiExecutePluginCommandRequest) RequestBody(requestBody map[string]map[string]interface{}) ApiExecutePluginCommandRequest {
	r.requestBody = &requestBody
	return r
}

func (r ApiExecutePluginCommandRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return r.ApiService.ExecutePluginCommandExecute(r)
}

/*
ExecutePluginCommand Execute command to the plugin instance

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param command
	@param pluginName
	@param projectName
	@return ApiExecutePluginCommandRequest
*/
func (a *PluginAPIService) ExecutePluginCommand(ctx context.Context, command, pluginName, projectName string) ApiExecutePluginCommandRequest {
	return ApiExecutePluginCommandRequest{
		ApiService:  a,
		ctx:         ctx,
		command:     command,
		pluginName:  pluginName,
		projectName: projectName,
	}
}

// Execute executes the request
//
//	@return map[string]interface{}
func (a *PluginAPIService) ExecutePluginCommandExecute(r ApiExecutePluginCommandRequest) (map[string]interface{}, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPut
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue map[string]interface{}
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.ExecutePluginCommand")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/{projectName}/{pluginName}/common/{command}"
	localVarPath = strings.Replace(localVarPath, "{"+"command"+"}", url.PathEscape(parameterValueToString(r.command, "command")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"pluginName"+"}", url.PathEscape(parameterValueToString(r.pluginName, "pluginName")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"projectName"+"}", url.PathEscape(parameterValueToString(r.projectName, "projectName")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.requestBody == nil {
		return localVarReturnValue, nil, reportError("requestBody is required and must be specified")
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
	localVarPostBody = r.requestBody
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

type ApiGetPluginsRequest struct {
	ctx        context.Context
	ApiService *PluginAPIService
}

func (r ApiGetPluginsRequest) Execute() ([]IntegrationTypeResource, *http.Response, error) {
	return r.ApiService.GetPluginsExecute(r)
}

/*
GetPlugins Get all available plugins

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@return ApiGetPluginsRequest
*/
func (a *PluginAPIService) GetPlugins(ctx context.Context) ApiGetPluginsRequest {
	return ApiGetPluginsRequest{
		ApiService: a,
		ctx:        ctx,
	}
}

// Execute executes the request
//
//	@return []IntegrationTypeResource
func (a *PluginAPIService) GetPluginsExecute(r ApiGetPluginsRequest) ([]IntegrationTypeResource, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []IntegrationTypeResource
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.GetPlugins")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin"

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

type ApiUpdatePluginStateRequest struct {
	ctx                 context.Context
	ApiService          *PluginAPIService
	pluginId            int64
	updatePluginStateRQ *UpdatePluginStateRQ
}

func (r ApiUpdatePluginStateRequest) UpdatePluginStateRQ(updatePluginStateRQ UpdatePluginStateRQ) ApiUpdatePluginStateRequest {
	r.updatePluginStateRQ = &updatePluginStateRQ
	return r
}

func (r ApiUpdatePluginStateRequest) Execute() (*OperationCompletionRS, *http.Response, error) {
	return r.ApiService.UpdatePluginStateExecute(r)
}

/*
UpdatePluginState Update ReportPortal plugin state

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param pluginId
	@return ApiUpdatePluginStateRequest
*/
func (a *PluginAPIService) UpdatePluginState(ctx context.Context, pluginId int64) ApiUpdatePluginStateRequest {
	return ApiUpdatePluginStateRequest{
		ApiService: a,
		ctx:        ctx,
		pluginId:   pluginId,
	}
}

// Execute executes the request
//
//	@return OperationCompletionRS
func (a *PluginAPIService) UpdatePluginStateExecute(r ApiUpdatePluginStateRequest) (*OperationCompletionRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPut
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *OperationCompletionRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.UpdatePluginState")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/{pluginId}"
	localVarPath = strings.Replace(localVarPath, "{"+"pluginId"+"}", url.PathEscape(parameterValueToString(r.pluginId, "pluginId")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.updatePluginStateRQ == nil {
		return localVarReturnValue, nil, reportError("updatePluginStateRQ is required and must be specified")
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
	localVarPostBody = r.updatePluginStateRQ
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

type ApiUploadPluginRequest struct {
	ctx        context.Context
	ApiService *PluginAPIService
	file       *os.File
}

func (r ApiUploadPluginRequest) File(file *os.File) ApiUploadPluginRequest {
	r.file = file
	return r
}

func (r ApiUploadPluginRequest) Execute() (*EntryCreatedRS, *http.Response, error) {
	return r.ApiService.UploadPluginExecute(r)
}

/*
UploadPlugin Upload new ReportPortal plugin

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@return ApiUploadPluginRequest
*/
func (a *PluginAPIService) UploadPlugin(ctx context.Context) ApiUploadPluginRequest {
	return ApiUploadPluginRequest{
		ApiService: a,
		ctx:        ctx,
	}
}

// Execute executes the request
//
//	@return EntryCreatedRS
func (a *PluginAPIService) UploadPluginExecute(r ApiUploadPluginRequest) (*EntryCreatedRS, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *EntryCreatedRS
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginAPIService.UploadPlugin")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.file == nil {
		return localVarReturnValue, nil, reportError("file is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"multipart/form-data"}

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
	var fileLocalVarFormFileName string
	var fileLocalVarFileName string
	var fileLocalVarFileBytes []byte

	fileLocalVarFormFileName = "file"
	fileLocalVarFile := r.file

	if fileLocalVarFile != nil {
		fbs, _ := io.ReadAll(fileLocalVarFile)

		fileLocalVarFileBytes = fbs
		fileLocalVarFileName = fileLocalVarFile.Name()
		fileLocalVarFile.Close()
		formFiles = append(formFiles, formFile{fileBytes: fileLocalVarFileBytes, fileName: fileLocalVarFileName, formFileName: fileLocalVarFormFileName})
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
