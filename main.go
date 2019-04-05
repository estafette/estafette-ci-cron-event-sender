package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

var (
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
		Str("app", "estafette-ci-cron-event-sender").
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

	// create client, in order to add headers
	client := pester.New()
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10
	request, err := http.NewRequest("POST", *ciServerCronEventsURL, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed creating http client")
	}

	// add headers
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", *apiKey))
	request.Header.Add("Content-Type", "application/json")

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		log.Error().Err(err).Str("logs", client.LogString()).Msgf("Failed sending event to %v", *ciServerCronEventsURL)
	}

	defer response.Body.Close()
}
