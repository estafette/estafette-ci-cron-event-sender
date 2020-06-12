package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

type ApiClient interface {
	GetToken(ctx context.Context, clientID, clientSecret string) (token string, err error)
	SendTick(ctx context.Context, token string) (err error)
}

// NewApiClient returns a new ApiClient
func NewApiClient(apiBaseURL string) ApiClient {
	return &apiClient{
		apiBaseURL: apiBaseURL,
	}
}

type apiClient struct {
	apiBaseURL string
}

func (c *apiClient) GetToken(ctx context.Context, clientID, clientSecret string) (token string, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::GetToken")
	defer span.Finish()

	clientObject := contracts.Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	bytes, err := json.Marshal(clientObject)
	if err != nil {
		return
	}

	getTokenURL := fmt.Sprintf("%v/api/auth/client/login", c.apiBaseURL)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	responseBody, err := c.postRequest(getTokenURL, span, strings.NewReader(string(bytes)), headers)

	tokenResponse := struct {
		Token string `json:"token"`
	}{}

	// unmarshal json body
	err = json.Unmarshal(responseBody, &tokenResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed unmarshalling get token response")
		return
	}

	return tokenResponse.Token, nil
}

func (c *apiClient) SendTick(ctx context.Context, token string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::SendTick")
	defer span.Finish()

	span.SetBaggageItem("tick-time", time.Now().UTC().Format(time.RFC3339))

	postCronEventURL := fmt.Sprintf("%v/api/integrations/cron/events", c.apiBaseURL)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	_, err = c.postRequest(postCronEventURL, span, nil, headers)

	return
}

func (c *apiClient) getRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("GET", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) postRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("POST", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) putRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("PUT", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) deleteRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("DELETE", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) makeRequest(method, uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {

	// create client, in order to add headers
	client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	request, err := http.NewRequest(method, uri, requestBody)
	if err != nil {
		return nil, err
	}

	// add tracing context
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

	// collect additional information on setting up connections
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add headers
	for k, v := range headers {
		request.Header.Add(k, v)
	}

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	ht.Finish()

	if len(allowedStatusCodes) == 0 {
		allowedStatusCodes = []int{http.StatusOK}
	}

	if !foundation.IntArrayContains(allowedStatusCodes, response.StatusCode) {
		return nil, fmt.Errorf("%v responded with status code %v", uri, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	return body, nil
}
