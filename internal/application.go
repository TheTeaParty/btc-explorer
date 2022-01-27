package internal

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/olivere/elastic/v7"
	"sources.witchery.io/packages/infrastructure/micro"
)

type Application struct {
	*micro.Application

	ElasticClient  *elastic.Client
	BtcDMainClient *rpcclient.Client
	BtcDTestClient *rpcclient.Client
}
