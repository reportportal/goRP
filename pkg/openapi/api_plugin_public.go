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

// PluginPublicAPIService PluginPublicAPI service
type PluginPublicAPIService service

type ApiExecutePublicPluginCommandRequest struct {
	ctx         context.Context
	ApiService  *PluginPublicAPIService
	command     string
	pluginName  string
	requestBody *map[string]interface{}
}

func (r ApiExecutePublicPluginCommandRequest) RequestBody(requestBody map[string]interface{}) ApiExecutePublicPluginCommandRequest {
	r.requestBody = &requestBody
	return r
}

func (r ApiExecutePublicPluginCommandRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return r.ApiService.ExecutePublicPluginCommandExecute(r)
}

/*
ExecutePublicPluginCommand Execute public command without authentication

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param command
	@param pluginName
	@return ApiExecutePublicPluginCommandRequest
*/
func (a *PluginPublicAPIService) ExecutePublicPluginCommand(ctx context.Context, command string, pluginName string) ApiExecutePublicPluginCommandRequest {
	return ApiExecutePublicPluginCommandRequest{
		ApiService: a,
		ctx:        ctx,
		command:    command,
		pluginName: pluginName,
	}
}

// Execute executes the request
//
//	@return map[string]interface{}
func (a *PluginPublicAPIService) ExecutePublicPluginCommandExecute(r ApiExecutePublicPluginCommandRequest) (map[string]interface{}, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPut
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue map[string]interface{}
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginPublicAPIService.ExecutePublicPluginCommand")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/public/{pluginName}/{command}"
	localVarPath = strings.Replace(localVarPath, "{"+"command"+"}", url.PathEscape(parameterValueToString(r.command, "command")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"pluginName"+"}", url.PathEscape(parameterValueToString(r.pluginName, "pluginName")), -1)

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

type ApiGetPlugins1Request struct {
	ctx        context.Context
	ApiService *PluginPublicAPIService
}

func (r ApiGetPlugins1Request) Execute() ([]IntegrationTypeResource, *http.Response, error) {
	return r.ApiService.GetPlugins1Execute(r)
}

/*
GetPlugins1 Get all available public plugins

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@return ApiGetPlugins1Request
*/
func (a *PluginPublicAPIService) GetPlugins1(ctx context.Context) ApiGetPlugins1Request {
	return ApiGetPlugins1Request{
		ApiService: a,
		ctx:        ctx,
	}
}

// Execute executes the request
//
//	@return []IntegrationTypeResource
func (a *PluginPublicAPIService) GetPlugins1Execute(r ApiGetPlugins1Request) ([]IntegrationTypeResource, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue []IntegrationTypeResource
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginPublicAPIService.GetPlugins1")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/public"

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

type ApiGetPublicFileRequest struct {
	ctx        context.Context
	ApiService *PluginPublicAPIService
	name       string
	pluginName string
}

func (r ApiGetPublicFileRequest) Execute() (*http.Response, error) {
	return r.ApiService.GetPublicFileExecute(r)
}

/*
GetPublicFile Get public plugin file without authentication

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param name
	@param pluginName
	@return ApiGetPublicFileRequest
*/
func (a *PluginPublicAPIService) GetPublicFile(ctx context.Context, name string, pluginName string) ApiGetPublicFileRequest {
	return ApiGetPublicFileRequest{
		ApiService: a,
		ctx:        ctx,
		name:       name,
		pluginName: pluginName,
	}
}

// Execute executes the request
func (a *PluginPublicAPIService) GetPublicFileExecute(r ApiGetPublicFileRequest) (*http.Response, error) {
	var (
		localVarHTTPMethod = http.MethodGet
		localVarPostBody   interface{}
		formFiles          []formFile
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PluginPublicAPIService.GetPublicFile")
	if err != nil {
		return nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/plugin/public/{pluginName}/file/{name}"
	localVarPath = strings.Replace(localVarPath, "{"+"name"+"}", url.PathEscape(parameterValueToString(r.name, "name")), -1)
	localVarPath = strings.Replace(localVarPath, "{"+"pluginName"+"}", url.PathEscape(parameterValueToString(r.pluginName, "pluginName")), -1)

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
	localVarHTTPHeaderAccepts := []string{}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarHTTPResponse, err
	}

	if a.client.cfg.ResponseMiddleware != nil {
		err = a.client.cfg.ResponseMiddleware(localVarHTTPResponse, localVarBody)
		if err != nil {
			return localVarHTTPResponse, err
		}
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarHTTPResponse, newErr
	}

	return localVarHTTPResponse, nil
}
