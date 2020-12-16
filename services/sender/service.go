package sender

import (
	"context"

	estafetteciapi "github.com/estafette/estafette-ci-cron-event-sender/clients/estafetteciapi"
	"github.com/opentracing/opentracing-go"
)

type Service interface {
	Init(ctx context.Context) (err error)
	Send(ctx context.Context) (err error)
}

func NewService(estafetteciapiClient estafetteciapi.Client) (Service, error) {
	return &service{
		estafetteciapiClient: estafetteciapiClient,
	}, nil
}

type service struct {
	estafetteciapiClient estafetteciapi.Client
}

func (s *service) Init(ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "sender.Service:Init")
	defer span.Finish()

	_, err = s.estafetteciapiClient.GetToken(ctx)
	if err != nil {
		return
	}

	return
}

func (s *service) Send(ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "sender.Service:Send")
	defer span.Finish()

	err = s.estafetteciapiClient.SendTick(ctx)
	if err != nil {
		return
	}

	return
}
