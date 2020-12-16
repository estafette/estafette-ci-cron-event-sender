package estafetteciapi

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

type Client interface {
	GetToken(ctx context.Context) (token string, err error)
	SendTick(ctx context.Context) (err error)
}

// NewClient returns a new estafetteciapi.Client
func NewClient(apiBaseURL, clientID, clientSecret string) (Client, error) {
	return &client{
		apiBaseURL:   apiBaseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}, nil
}

type client struct {
	apiBaseURL   string
	clientID     string
	clientSecret string
	token        string
}

func (c *client) GetToken(ctx context.Context) (token string, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "estafetteciapi.Client:GetToken")
	defer span.Finish()

	log.Debug().Msgf("Retrieving JWT token")

	clientObject := contracts.Client{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
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

	// set token
	c.token = tokenResponse.Token

	return tokenResponse.Token, nil
}

func (c *client) SendTick(ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "estafetteciapi.Client:SendTick")
	defer span.Finish()

	span.SetBaggageItem("tick-time", time.Now().UTC().Format(time.RFC3339))

	postCronEventURL := fmt.Sprintf("%v/api/integrations/cron/events", c.apiBaseURL)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", c.token),
		"Content-Type":  "application/json",
	}

	_, err = c.postRequest(postCronEventURL, span, nil, headers)

	return
}

func (c *client) getRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("GET", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *client) postRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("POST", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *client) putRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("PUT", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *client) deleteRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("DELETE", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *client) makeRequest(method, uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {

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
