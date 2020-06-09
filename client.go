package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

func getToken(ctx context.Context, getTokenURL, clientID, clientSecret string) (token string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "GetToken")
	defer span.Finish()

	clientObject := contracts.Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	bytes, err := json.Marshal(clientObject)
	if err != nil {
		return
	}

	// create client, in order to add headers
	client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	request, err := http.NewRequest("POST", getTokenURL, strings.NewReader(string(bytes)))
	if err != nil {
		return "", err
	}

	// add tracing context
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

	// collect additional information on setting up connections
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add headers
	request.Header.Add("Content-Type", "application/json")

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	ht.Finish()

	if response.StatusCode != http.StatusOK {
		return "keysResponse", fmt.Errorf("%v responded with status code %v", getTokenURL, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	tokenResponse := struct {
		Token string `json:"token"`
	}{}

	// unmarshal json body
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(body)).Msgf("Failed unmarshalling get token response")
		return
	}

	return tokenResponse.Token, nil
}

func sendTick(ctx context.Context, cronURL, token string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "SendTick")
	defer span.Finish()

	span.SetBaggageItem("tick-time", time.Now().UTC().Format(time.RFC3339))

	// create client, in order to add headers
	client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	request, err := http.NewRequest("POST", cronURL, nil)
	if err != nil {
		return err
	}

	// add tracing context
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

	// collect additional information on setting up connections
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add headers
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))
	request.Header.Add("Content-Type", "application/json")

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	ht.Finish()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed sending event to %v, response status code %v", cronURL, response.StatusCode)
	}

	return nil
}
