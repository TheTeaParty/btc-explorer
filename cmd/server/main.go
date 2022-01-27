package main

import (
	"context"
	"fmt"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/olivere/elastic/v7"
	"github.com/olivere/elastic/v7/config"
	"go.uber.org/zap"
	"sources.witchery.io/coven/btc/internal"
	"sources.witchery.io/coven/btc/internal/handler"
	"sources.witchery.io/coven/btc/internal/sync"
	btc "sources.witchery.io/coven/btc/pkg"
	btcAPI "sources.witchery.io/coven/btc/pkg/api/proto"
	"sources.witchery.io/packages/infrastructure/env"
	"sources.witchery.io/packages/infrastructure/micro"
)

func main() {

	app := &internal.Application{
		Application: micro.NewService(btc.AppName, btc.ServiceName),
	}

	// Handle panics
	defer func() {
		_ = app.Logger.Sync()
		if err := recover(); err != nil {
			app.Logger.Error("Service panicked",
				zap.Error(err.(error)))
		}
	}()

	sniff := false

	elConf := &config.Config{
		URL:   env.GenEnv("ELASTIC_HOST", "http://localhost:9200"),
		Sniff: &sniff,
	}
	elasticClient, err := elastic.NewClientFromConfig(elConf)
	if err != nil {
		app.Logger.Fatal("Error connecting elasticsearch",
			zap.Error(err))
	}

	app.ElasticClient = elasticClient

	btcdHandler := handler.NewBtcDHandler(app)
	connMainCfg := &rpcclient.ConnConfig{
		Host:         env.GenEnv("BTC_MAIN_HOST", ""),
		Endpoint:     env.GenEnv("BTC_MAIN_ENDPOINT", ""),
		User:         env.GenEnv("BTC_MAIN_USER", ""),
		Pass:         env.GenEnv("BTC_MAIN_PASS", ""),
		Certificates: []byte(env.GenEnv("BTC_MAIN_CERT", "")),
	}
	connTestCfg := &rpcclient.ConnConfig{
		Host:         env.GenEnv("BTC_TEST_HOST", ""),
		Endpoint:     env.GenEnv("BTC_TEST_ENDPOINT", ""),
		User:         env.GenEnv("BTC_TEST_USER", ""),
		Pass:         env.GenEnv("BTC_TEST_PASS", ""),
		Certificates: []byte(env.GenEnv("BTC_TEST_CERT", "")),
	}

	btcdMainClient, err := rpcclient.New(connMainCfg, btcdHandler)
	if err != nil {
		app.Logger.Fatal("Error connecting to btc main node",
			zap.Error(err))
	}

	btcdTestClient, err := rpcclient.New(connTestCfg, btcdHandler)
	if err != nil {
		app.Logger.Fatal("Error connecting to btc test node",
			zap.Error(err))
	}

	app.BtcDMainClient = btcdMainClient
	app.BtcDTestClient = btcdTestClient

	ctx := context.Background()
	//
	//// main network syncer
	//go func(ctx context.Context, app *internal.Application) {
	//	syncer := sync.NewB2ESync(app, "main")
	//	if err := syncer.Sync(ctx); err != nil {
	//		app.Logger.Fatal("Error syncing main network",
	//			zap.Error(err))
	//	}
	//}(ctx, app)
	//
	// test network syncer
	go func(ctx context.Context, app *internal.Application) {
		syncer := sync.NewB2ESync(app, "test")
		if err := syncer.Sync(ctx); err != nil {
			app.Logger.Fatal("Error syncing test network",
				zap.Error(err))
		}
	}(ctx, app)

	fmt.Println(env.GenEnv("NATS_HOST", ""))
	if err := app.ConnectBroker(); err != nil {
		app.Logger.Fatal("Error connecting to broker",
			zap.Error(err))
	}

	serviceHandler := handler.NewServiceHandler(app)
	if err := btcAPI.RegisterBTCServiceHandler(app.Service.Server(), serviceHandler); err != nil {
		app.Logger.Fatal("Error registering service",
			zap.Error(err))
	}

	if err := app.Run(); err != nil {
		app.Logger.Fatal("Error running service",
			zap.Error(err))
	}
}
