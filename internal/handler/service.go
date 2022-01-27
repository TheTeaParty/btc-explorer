package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/olivere/elastic/v7"
	"sources.witchery.io/coven/btc/internal"
	btcAPI "sources.witchery.io/coven/btc/pkg/api/proto"
	"strconv"
)

type serviceHandler struct {
	app *internal.Application
}

func (s *serviceHandler) GetBlocks(ctx context.Context, request *btcAPI.BlockRequest, response *btcAPI.RawResponse) error {
	es := s.app.ElasticClient

	var q elastic.Query
	q = elastic.NewMatchAllQuery()

	result, err := es.Search().Index(request.Network+"-block").
		Sort("height", false).Size(35).Query(q).Do(ctx)
	if err != nil {
		return err
	}

	blocks := make([]map[string]interface{}, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		var j map[string]interface{}
		if err := json.Unmarshal(hit.Source, &j); err != nil {
			return err
		}
		delete(j, "tx")
		blocks[i] = j
	}

	if response.RawBody, err = json.Marshal(blocks); err != nil {
		return err
	}

	return nil
}

func (s *serviceHandler) GetOutputs(ctx context.Context, request *btcAPI.OutputRequest, response *btcAPI.RawResponse) error {
	es := s.app.ElasticClient

	var q elastic.Query
	q = elastic.NewMatchAllQuery()

	if request.Address != "" {
		q = elastic.NewBoolQuery().
			Must(
				elastic.NewMatchQuery("coinbase", "false"),
				elastic.NewMatchQuery("addresses", request.Address),
			)
	}

	result, err := es.Search().Index(request.Network+"-vout").Sort("time", false).
		Size(2000).Query(q).Do(ctx)
	if err != nil {
		return err
	}

	txs := make([]map[string]interface{}, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		var j map[string]interface{}
		if err := json.Unmarshal(hit.Source, &j); err != nil {
			return err
		}
		txs[i] = j
	}

	if response.RawBody, err = json.Marshal(txs); err != nil {
		return err
	}

	return nil

}

func (s *serviceHandler) GetTxs(ctx context.Context, request *btcAPI.TxsRequest, response *btcAPI.RawResponse) error {
	es := s.app.ElasticClient

	var q elastic.Query
	q = elastic.NewMatchAllQuery()

	if request.Address != "" {
		q = elastic.NewBoolQuery().
			Should(
				elastic.NewMatchQuery("vins.address", request.Address),
				elastic.NewMatchQuery("vouts.address", request.Address),
			)
	}

	result, err := es.Search().Index(request.Network + "-tx").Size(35).
		Sort("time", false).Query(q).Do(ctx)
	if err != nil {
		return err
	}

	txs := make([]map[string]interface{}, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		var j map[string]interface{}
		if err := json.Unmarshal(hit.Source, &j); err != nil {
			return err
		}
		txs[i] = j
	}

	if response.RawBody, err = json.Marshal(txs); err != nil {
		return err
	}

	return nil
}

func (s *serviceHandler) GetBlock(ctx context.Context,
	request *btcAPI.BlockRequest, response *btcAPI.RawResponse) error {

	es := s.app.ElasticClient

	q := elastic.NewMatchQuery("hash", request.HashOrHeight)
	if _, err := strconv.Atoi(request.HashOrHeight); err == nil {
		q = elastic.NewMatchQuery("height", request.HashOrHeight)
	}

	result, err := es.Search().Index(request.Network + "-block").Query(q).Do(ctx)
	if err != nil {
		return err
	}

	hits := result.Hits.Hits
	if len(hits) == 0 {
		return errors.New("block not found")
	}

	response.RawBody = hits[0].Source

	return nil

}

func (s *serviceHandler) GetTx(ctx context.Context,
	request *btcAPI.TxRequest, response *btcAPI.RawResponse) error {

	es := s.app.ElasticClient

	q := elastic.NewMatchQuery("txid", request.TxID)

	result, err := es.Search().Index(request.Network + "-tx").Query(q).Do(ctx)
	if err != nil {
		return err
	}

	hits := result.Hits.Hits
	if len(hits) == 0 {
		return errors.New("transaction not found")
	}

	response.RawBody = hits[0].Source

	return nil

}

func (s *serviceHandler) GetBalance(ctx context.Context,
	request *btcAPI.BalanceRequest, response *btcAPI.RawResponse) error {

	es := s.app.ElasticClient

	q := elastic.NewMatchQuery("address", request.Address)

	result, err := es.Search().Index(request.Network + "-balance").Query(q).Do(ctx)
	if err != nil {
		return err
	}

	hits := result.Hits.Hits
	if len(hits) == 0 {
		return errors.New("address not found")
	}

	response.RawBody = hits[0].Source

	return nil

}

func (s *serviceHandler) GetBalanceJournal(ctx context.Context,
	request *btcAPI.BalanceJournalRequest, response *btcAPI.RawResponse) error {

	es := s.app.ElasticClient

	q := elastic.NewMatchQuery("address", request.Address)

	result, err := es.Search().Index(request.Network + "-balancejournal").Query(q).Do(ctx)
	if err != nil {
		return err
	}

	journal := make([]map[string]interface{}, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		var j map[string]interface{}
		if err := json.Unmarshal(hit.Source, &j); err != nil {
			return err
		}
		journal[i] = j
	}

	if response.RawBody, err = json.Marshal(journal); err != nil {
		return err
	}

	return nil
}

func NewServiceHandler(app *internal.Application) btcAPI.BTCServiceHandler {
	return &serviceHandler{app: app}
}
