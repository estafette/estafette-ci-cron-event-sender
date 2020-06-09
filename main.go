package main

import (
	"context"
	"io"
	"runtime"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
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

	getTokenURL           = kingpin.Flag("get-token-url", "The endpoint on the estafette-ci-api to retrieve a client token from").Envar("GET_TOKEN_URL").String()
	clientID              = kingpin.Flag("client-id", "The id of the client as configured in Estafette, to securely communicate with the api.").Envar("CLIENT_ID").String()
	clientSecret          = kingpin.Flag("client-secret", "The secret of the client as configured in Estafette, to securely communicate with the api.").Envar("CLIENT_SECRET").String()
	ciServerCronEventsURL = kingpin.Flag("cron-events-url", "The endpoint on the estafette-ci-api to post the event to").Envar("CRON_EVENTS_URL").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(foundation.NewApplicationInfo(appgroup, app, version, branch, revision, buildDate))

	closer := initJaeger(app)
	defer closer.Close()

	ctx := context.Background()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Main")
	defer span.Finish()

	token, err := getToken(ctx, *getTokenURL, *clientID, *clientSecret)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed retrieving JWT token")
	}

	err = sendTick(ctx, *ciServerCronEventsURL, token)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed sending tick")
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
