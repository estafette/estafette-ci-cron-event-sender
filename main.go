package main

import (
	"context"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	sender "github.com/estafette/estafette-ci-cron-event-sender/services/sender"
	manifest "github.com/estafette/estafette-ci-manifest"
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

	queueHosts   = kingpin.Flag("queue-hosts", "The list of queue servers to publish to").Default("estafette-ci-queue-0.estafette-ci-queue").OverrideDefaultFromEnvar("QUEUE_HOSTS").String()
	queueSubject = kingpin.Flag("queue-subject", "The queue subject name to publish to").Default("cron").OverrideDefaultFromEnvar("QUEUE_SUBJECT").String()
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

	senderService, err := sender.NewService()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed creating sender.Service")
	}

	err = senderService.CreateConnection(ctx, strings.Split(*queueHosts, ","))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed creating connection to nats")
	}
	defer senderService.CloseConnection(ctx)

	err = senderService.Publish(ctx, *queueSubject, manifest.EstafetteCronEvent{Time: time.Now().UTC()})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed publishing cron event")
	}
}

func handleError(jaegerCloser io.Closer, err error, message string) {
	if err != nil {
		jaegerCloser.Close()
		log.Fatal().Err(err).Msg(message)
	}
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
