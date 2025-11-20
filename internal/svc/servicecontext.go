package svc

import (
	"fmt"

	"gocerery/internal/config"

	"github.com/gocelery/gocelery"
)

type ServiceContext struct {
	Config        config.Config
	CeleryClient  *gocelery.CeleryClient
	CeleryBackend *gocelery.RedisCeleryBackend
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	ctx := &ServiceContext{
		Config: c,
	}

	if c.Celery.Broker != "" && c.Celery.Backend != "" {
		broker := gocelery.NewRedisCeleryBroker(c.Celery.Broker)
		backend := gocelery.NewRedisCeleryBackend(c.Celery.Backend)
		workers := c.Celery.Workers
		if workers <= 0 {
			workers = 1
		}

		client, err := gocelery.NewCeleryClient(broker, backend, workers)
		if err != nil {
			return nil, fmt.Errorf("init celery client: %w", err)
		}

		ctx.CeleryClient = client
		ctx.CeleryBackend = backend
	}

	return ctx, nil
}

// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2
