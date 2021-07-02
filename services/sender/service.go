package sender

import (
	"context"
	"strings"

	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/nats-io/nats.go"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
)

type Service interface {
	CreateConnection(ctx context.Context, hosts []string) (err error)
	CloseConnection(ctx context.Context)
	Publish(ctx context.Context, subject string, cronEvent manifest.EstafetteCronEvent) (err error)
}

func NewService() (Service, error) {
	return &service{}, nil
}

type service struct {
	natsConnection        *nats.Conn
	natsEncodedConnection *nats.EncodedConn
}

func (s *service) CreateConnection(ctx context.Context, hosts []string) (err error) {
	s.natsConnection, err = nats.Connect(strings.Join(hosts, ","))
	if err != nil {
		return
	}

	s.natsEncodedConnection, err = nats.NewEncodedConn(s.natsConnection, nats.JSON_ENCODER)
	if err != nil {
		return
	}

	return nil
}

func (s *service) CloseConnection(ctx context.Context) {
	if s.natsEncodedConnection != nil {
		s.natsEncodedConnection.Close()
	}
	if s.natsConnection != nil {
		s.natsConnection.Close()
	}
}

func (s *service) Publish(ctx context.Context, subject string, cronEvent manifest.EstafetteCronEvent) (err error) {

	log.Info().Msgf("Publishing cron event to queue with subject %v", subject)

	span, _ := opentracing.StartSpanFromContext(ctx, "sender:Send")
	defer span.Finish()

	err = s.natsEncodedConnection.Publish(subject, &cronEvent)
	if err != nil {
		return
	}

	return
}
