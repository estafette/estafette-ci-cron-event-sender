package main

import (
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var (
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

	// log as severity for stackdriver logging to recognize the level
	zerolog.LevelFieldName = "severity"

	// set some default fields added to all logs
	log.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", app).
		Str("version", version).
		Logger()

	// use zerolog for any logs sent via standard log library
	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)

	// log startup message
	log.Info().
		Str("branch", branch).
		Str("revision", revision).
		Str("buildDate", buildDate).
		Str("goVersion", goVersion).
		Msg("Starting estafette-ci-cron-event-sender...")

	// parse command line parameters
	kingpin.Parse()

	closer := initJaeger(app)
	defer closer.Close()

	span := opentracing.StartSpan("SendTick")
	defer span.Finish()

	span.LogEvent("set-bagage")
	span.SetBaggageItem("tick-time", time.Now().UTC().Format(time.RFC3339))

	// create client, in order to add headers
	span.LogEvent("create-http-client")

	//client := pester.NewExtendedClient(nethttp.Client{})
	client := pester.New()
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	span.LogEvent("create-http-request")
	request, err := http.NewRequest("POST", *ciServerCronEventsURL, nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed creating http client")
	}

	// add tracing context
	span.LogEvent("set-tracing-context")
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add tracing context
	// ext.SpanKindRPCClient.Set(span)
	// ext.HTTPMethod.Set(span, request.Method)
	// ext.HTTPUrl.Set(span, request.URL.String())
	// span.Tracer().Inject(
	// 	span.Context(),
	// 	opentracing.HTTPHeaders,
	// 	opentracing.HTTPHeadersCarrier(request.Header),
	// )

	// add headers
	span.LogEvent("add-request-headers")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", *apiKey))
	request.Header.Add("Content-Type", "application/json")

	// perform actual request
	span.LogEvent("do-http-request")
	response, err := client.Do(request)
	if err != nil {
		log.Fatal().Err(err).Str("logs", client.LogString()).Msgf("Failed sending event to %v", *ciServerCronEventsURL)
	}
	defer response.Body.Close()
	span.LogEvent("finish-http-request")
	ht.Finish()

	ext.HTTPStatusCode.Set(span, uint16(response.StatusCode))

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
