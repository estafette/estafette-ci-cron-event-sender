package main

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	apiKey                = kingpin.Flag("api-key", "The Estafette server passes in this json structure to parameterize the build, set trusted images and inject credentials.").Envar("API_KEY").String()
	ciServerCronEventsURL = kingpin.Flag("cron-events-url", "The endpoint on the estafette-ci-api to post the event to").Envar("CRON_EVENTS_URL").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(appgroup, app, version, branch, revision, buildDate)

	closer := initJaeger(app)
	defer closer.Close()

	span := opentracing.StartSpan("SendTick")
	defer span.Finish()

	span.SetBaggageItem("tick-time", time.Now().UTC().Format(time.RFC3339))

	// create client, in order to add headers
	client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	request, err := http.NewRequest("POST", *ciServerCronEventsURL, nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed creating http client")
	}

	// add tracing context
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

	// collect additional information on setting up connections
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add headers
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", *apiKey))
	request.Header.Add("Content-Type", "application/json")

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		log.Fatal().Err(err).Str("logs", client.LogString()).Msgf("Failed sending event to %v", *ciServerCronEventsURL)
	}
	defer response.Body.Close()
	ht.Finish()

	if response.StatusCode != http.StatusOK {
		log.Fatal().Err(err).Str("logs", client.LogString()).Msgf("Failed sending event to %v, response status code %v", *ciServerCronEventsURL, response.StatusCode)
	}

	log.Info().Msgf("Sent tick succesfully to %v...", *ciServerCronEventsURL)
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func initJaeger(service string) io.Closer {

	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger config from environment variables failed")
	}

	closer, err := cfg.InitGlobalTracer(service, jaegercfg.Logger(jaeger.StdLogger))

	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger tracer failed")
	}

	return closer
}
