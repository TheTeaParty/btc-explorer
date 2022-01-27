package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/olivere/elastic/v7"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"sources.witchery.io/coven/btc/internal"
	"sources.witchery.io/coven/btc/internal/data"
	"sources.witchery.io/coven/btc/internal/domain"
	"sources.witchery.io/coven/btc/internal/util"
	"strconv"
	"strings"
	"time"
)

const (
	rollback = 5
)

type b2eSync struct {
	network    string
	app        *internal.Application
	btcdClient *rpcclient.Client
}

func (s *b2eSync) bulkInsertBalanceJournalQuery(ctx context.Context,
	balanceWithIDs []domain.AddressWithAmountAndTxid, operation string) error {

	es := s.app.ElasticClient

	p, err := es.BulkProcessor().Name(s.network + "-BulkInsertBalanceJournal").Workers(5).BulkActions(40000).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("error creating bulk processor %v", err)
	}

	for _, balance := range balanceWithIDs {
		elBalanceJournal := domain.NewBalanceJournalFun(balance.Address, operation, balance.Txid, balance.Amount)
		insertBalanceJournal := elastic.NewBulkIndexRequest().Index(s.network + "-balancejournal").Doc(elBalanceJournal)
		p.Add(insertBalanceJournal)
	}

	return p.Close()
}

func (s *b2eSync) bulkBalanceQueryChunk(ctx context.Context, addresses ...interface{}) ([]*domain.BalanceWithID, error) {

	var (
		balanceWithIDs []*domain.BalanceWithID
		qAddresses     []interface{}
	)

	es := s.app.ElasticClient
	uAddresses := util.RemoveSliceDuplicates(addresses...)

	for _, a := range uAddresses {
		qAddresses = append(qAddresses, a)
	}

	q := elastic.NewTermsQuery("address", qAddresses...)

	searchResult, err := es.Search().Index(s.network + "-balance").Size(len(qAddresses)).Query(q).Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, balance := range searchResult.Hits.Hits {
		var elBalance domain.Balance
		if err := json.Unmarshal(balance.Source, &elBalance); err != nil {
			return nil, err
		}

		balanceWithIDs = append(balanceWithIDs, &domain.BalanceWithID{
			ID:      balance.Id,
			Balance: elBalance,
		})
	}

	return balanceWithIDs, nil
}

func (s *b2eSync) bulkBalanceQuery(ctx context.Context, addresses ...interface{}) ([]*domain.BalanceWithID, error) {

	uniqueAddresses := util.RemoveSliceDuplicates(addresses...)

	var qAddresses []interface{}
	for _, address := range uniqueAddresses {
		qAddresses = append(qAddresses, address)
	}

	var (
		balanceWithIDs []*domain.BalanceWithID
		addressesTmp   []interface{}
	)

	for len(qAddresses) >= 500 {
		addressesTmp, qAddresses = qAddresses[:500], qAddresses[500:]

		balanceWithIDsTmp, err := s.bulkBalanceQueryChunk(ctx, addressesTmp...)
		if err != nil {
			return nil, err
		}

		balanceWithIDs = append(balanceWithIDs, balanceWithIDsTmp...)
	}

	if len(qAddresses) > 0 {
		balanceWithIDsTmp, err := s.bulkBalanceQueryChunk(ctx, qAddresses...)
		if err != nil {
			return nil, err
		}

		balanceWithIDs = append(balanceWithIDs, balanceWithIDsTmp...)
	}

	return balanceWithIDs, nil
}

func (s *b2eSync) rollBackTxVoutBalance(ctx context.Context, block *btcjson.GetBlockVerboseResult) error {

	es := s.app.ElasticClient
	bulkRequest := es.Bulk()

	var (
		vinAddresses                      []interface{}
		voutAddresses                     []interface{}
		vinAddressWithAmountSlice         []domain.Balance
		voutAddressWithAmountSlice        []domain.Balance
		voutAddressWithAmountAndTxidSlice []domain.AddressWithAmountAndTxid
		vinAddressWithAmountAndTxidSlice  []domain.AddressWithAmountAndTxid
		UniqueVinAddressesWithSumWithdraw []*domain.AddressWithAmount
		UniqueVoutAddressesWithSumDeposit []*domain.AddressWithAmount
		vinBalancesWithIDs                []*domain.BalanceWithID
		voutBalancesWithIDs               []*domain.BalanceWithID
	)

	if err := s.DeleteEsTxsByBlockHash(ctx, block.Hash); err != nil {
		return fmt.Errorf("error remove tx by block hash %v", err)
	}

	for _, tx := range block.RawTx {

		voutWithIDSliceForVins, _ := s.QueryVoutsByUsedFieldAndBelongTxID(ctx, tx.Vin, tx.Txid)

		for _, voutWithID := range voutWithIDSliceForVins {
			updateVoutUsedField := elastic.NewBulkUpdateRequest().Index(s.network + "-vout").Type("vout").Id(voutWithID.ID).
				Doc(map[string]interface{}{"used": nil})
			bulkRequest.Add(updateVoutUsedField).Refresh("true")

			_, vinAddressesTmp, vinAddressWithAmountSliceTmp, vinAddressWithAmountAndTxidSliceTmp := util.ParseESVout(voutWithID, tx.Txid)
			vinAddresses = append(vinAddresses, vinAddressesTmp...)
			vinAddressWithAmountSlice = append(vinAddressWithAmountSlice, vinAddressWithAmountSliceTmp...)
			vinAddressWithAmountAndTxidSlice = append(vinAddressWithAmountAndTxidSlice, vinAddressWithAmountAndTxidSliceTmp...)
		}

		indexVouts := util.IndexedVouts(tx.Vout, tx.Txid)

		voutWithIDSliceForVouts, err := s.QueryVoutWithVinsOrVouts(ctx, indexVouts)
		if err != nil {
			return err
		}
		for _, voutWithID := range voutWithIDSliceForVouts {
			deleteVout := elastic.NewBulkDeleteRequest().Index(s.network + "-vout").Type("vout").Id(voutWithID.ID)
			bulkRequest.Add(deleteVout).Refresh("true")

			_, voutAddressesTmp, voutAddressWithAmountSliceTmp, voutAddressWithAmountAndTxidSliceTmp :=
				util.ParseESVout(voutWithID, tx.Txid)
			voutAddresses = append(voutAddresses, voutAddressesTmp...)
			voutAddressWithAmountSlice = append(voutAddressWithAmountSlice, voutAddressWithAmountSliceTmp...)
			voutAddressWithAmountAndTxidSlice = append(voutAddressWithAmountAndTxidSlice,
				voutAddressWithAmountAndTxidSliceTmp...)
		}
	}

	UniqueVinAddressesWithSumWithdraw = util.CalcUniqueAddressSumForVinVout(vinAddresses, vinAddressWithAmountSlice)
	bulkQueryVinBalance, err := s.bulkBalanceQuery(ctx, vinAddresses...)
	if err != nil {
		return err
	}
	vinBalancesWithIDs = bulkQueryVinBalance

	UniqueVoutAddressesWithSumDeposit = util.CalcUniqueAddressSumForVinVout(voutAddresses, voutAddressWithAmountSlice)
	bulkQueryVoutBalance, err := s.bulkBalanceQuery(ctx, voutAddresses...)
	if err != nil {
		return err
	}
	voutBalancesWithIDs = bulkQueryVoutBalance

	bulkUpdateVinBalanceRequest := es.Bulk()

	for _, vinAddressWithSumWithdraw := range UniqueVinAddressesWithSumWithdraw {
		for _, vinBalanceWithID := range vinBalancesWithIDs {
			if vinAddressWithSumWithdraw.Address == vinBalanceWithID.Balance.Address {
				balance := decimal.NewFromFloat(vinBalanceWithID.Balance.Amount).Add(vinAddressWithSumWithdraw.Amount)
				amount, _ := balance.Float64()
				updateVinBalance := elastic.NewBulkUpdateRequest().Index(s.network + "-balance").Type("balance").Id(vinBalanceWithID.ID).
					Doc(map[string]interface{}{"amount": amount})
				bulkUpdateVinBalanceRequest.Add(updateVinBalance).Refresh("true")
				break
			}
		}
	}
	if bulkUpdateVinBalanceRequest.NumberOfActions() != 0 {
		bulkUpdateVinBalanceResp, err := bulkUpdateVinBalanceRequest.Refresh("true").Do(ctx)
		if err != nil {
			return err
		}
		bulkUpdateVinBalanceResp.Updated()
	}

	for _, voutAddressWithSumDeposit := range UniqueVoutAddressesWithSumDeposit {
		for _, voutBalanceWithID := range voutBalancesWithIDs {
			if voutAddressWithSumDeposit.Address == voutBalanceWithID.Balance.Address {
				balance := decimal.NewFromFloat(voutBalanceWithID.Balance.Amount).Sub(voutAddressWithSumDeposit.Amount)
				amount, _ := balance.Float64()
				updateVinBalance := elastic.NewBulkUpdateRequest().Index(s.network + "-balance").
					Type("balance").Id(voutBalanceWithID.ID).
					Doc(map[string]interface{}{"amount": amount})
				bulkRequest.Add(updateVinBalance).Refresh("true")
				break
			}
		}
	}

	if bulkRequest.NumberOfActions() != 0 {
		bulkResp, err := bulkRequest.Refresh("true").Do(ctx)
		if err != nil {
			return err
		}
		bulkResp.Updated()
		bulkResp.Deleted()
		bulkResp.Indexed()
	}

	if err := s.bulkInsertBalanceJournalQuery(ctx, voutAddressWithAmountAndTxidSlice, "rollback-");
		err != nil {
		return err
	}

	if err := s.bulkInsertBalanceJournalQuery(ctx, vinAddressWithAmountAndTxidSlice, "rollback+");
		err != nil {
		return err
	}

	return nil
}

func (s *b2eSync) dumpTxVoutBalance(ctx context.Context, block *btcjson.GetBlockVerboseResult) error {

	es := s.app.ElasticClient
	bulkRequest := es.Bulk()

	var (
		voutAddresses                  []interface{}
		vinAddresses                   []interface{}
		vastAddressesWithAmount        []domain.Balance
		vinAddressesWithAmount         []domain.Balance
		vinBalanceWithIDs              []*domain.BalanceWithID
		voutBalanceWithIDs             []*domain.BalanceWithID
		vastAddressesWithAmountAndTxID []domain.AddressWithAmountAndTxid
		vinAddressesWithAmountAndTxID  []domain.AddressWithAmountAndTxid
	)

	for _, tx := range block.RawTx {
		var (
			voutAmount   decimal.Decimal
			vinAmount    decimal.Decimal
			fee          decimal.Decimal
			txVoutsField []domain.AddressWithValueInTx
			txtVinsField []domain.AddressWithValueInTx
		)

		for _, vout := range tx.Vout {

			elVout, err := domain.NewVout(vout, tx.Vin, tx.Txid)
			if err != nil {
				continue
			}

			createdVout := elastic.NewBulkIndexRequest().Index(s.network + "-vout").Doc(elVout)
			bulkRequest.Add(createdVout).Refresh("true")

			voutAmount = voutAmount.Add(decimal.NewFromFloat(vout.Value))

			for _, address := range vout.ScriptPubKey.Addresses {
				txVoutsField = append(txVoutsField, domain.AddressWithValueInTx{
					Address: address,
					Value:   vout.Value,
				})

				voutAddresses = append(voutAddresses, address)
				vastAddressesWithAmount = append(vastAddressesWithAmount, domain.Balance{
					Address: address,
					Amount:  vout.Value,
				})

				vastAddressesWithAmountAndTxID = append(vastAddressesWithAmountAndTxID, domain.AddressWithAmountAndTxid{
					Address: address,
					Amount:  vout.Value,
					Txid:    tx.Txid,
				})
			}

		}

		indexedVins := util.IndexVins(tx.Vin)
		voutWithIDs, err := s.QueryVoutWithVinsOrVouts(ctx, indexedVins)
		if err != nil {
			return fmt.Errorf("error getting vout with ids %v", err)
		}

		for _, voutWithID := range voutWithIDs {

			vinAmount = vinAmount.Add(decimal.NewFromFloat(voutWithID.Vout.Value))

			updateVoutUsedField := elastic.NewBulkUpdateRequest().Index(s.network + "-vout").Id(voutWithID.ID).
				Doc(map[string]interface{}{
					"used": domain.VoutUsed{
						Txid:     tx.Txid,
						VinIndex: voutWithID.Vout.Voutindex,
					},
				})
			bulkRequest.Add(updateVoutUsedField).Refresh("true")

			for _, address := range voutWithID.Vout.Addresses {
				vinAddresses = append(vinAddresses, address)
				vinAddressesWithAmount = append(vinAddressesWithAmount, domain.Balance{
					Address: address,
					Amount:  voutWithID.Vout.Value,
				})

				txtVinsField = append(txtVinsField, domain.AddressWithValueInTx{
					Address: address,
					Value:   voutWithID.Vout.Value,
				})

				vinAddressesWithAmountAndTxID = append(vinAddressesWithAmountAndTxID, domain.AddressWithAmountAndTxid{
					Address: address,
					Amount:  voutWithID.Vout.Value,
					Txid:    tx.Txid,
				})

			}
		}

		fee = vinAmount.Sub(voutAmount)
		if len(tx.Vin) == 1 && len(tx.Vin[0].Coinbase) != 0 &&
			len(tx.Vin[0].Txid) == 0 || vinAmount.Equal(voutAmount) {
			fee = decimal.NewFromFloat(0)
		}

		elFee, _ := fee.Float64()
		txBulk := domain.NewElTx(tx, elFee, block.Hash, txtVinsField, txVoutsField)
		insertTx := elastic.NewBulkIndexRequest().Index(s.network + "-tx").Doc(txBulk)
		bulkRequest.Add(insertTx).Refresh("true")
	}

	uVinAddressesWithSumWithdraw := util.CalcUniqueAddressSumForVinVout(vinAddresses, vinAddressesWithAmount)
	vinBalanceWithIDs, err := s.bulkBalanceQuery(ctx, vinAddresses...)

	uAddresses := util.RemoveSliceDuplicates(vinAddresses...)
	if len(uAddresses) != len(vinBalanceWithIDs) {
		// @todo HUH???
		return errors.New("duplicate addresses found")
	}

	bulkUpdateVinBalanceRequest := es.Bulk()

	for _, vinAddressesWithWithdraw := range uVinAddressesWithSumWithdraw {
		for _, vinBalanceWithID := range vinBalanceWithIDs {
			if vinAddressesWithWithdraw.Address == vinBalanceWithID.Balance.Address {
				balance := decimal.NewFromFloat(vinBalanceWithID.Balance.Amount).Sub(vinAddressesWithWithdraw.Amount)
				amount, _ := balance.Float64()
				updateVinBalance := elastic.NewBulkUpdateRequest().Index(s.network + "-balance").Id(vinBalanceWithID.ID).
					Doc(map[string]interface{}{"amount": amount})
				bulkUpdateVinBalanceRequest.Add(updateVinBalance).Refresh("true")
				break
			}
		}
	}

	if bulkUpdateVinBalanceRequest.NumberOfActions() != 0 {
		bulkUpdateVinBalanceResponse, err := bulkUpdateVinBalanceRequest.Refresh("true").Do(ctx)
		if err != nil {
			return fmt.Errorf("error bulk update vin balances %v", err)
		}

		bulkUpdateVinBalanceResponse.Updated()
	}

	uVoutAddressesWithSumDeposit := util.CalcUniqueAddressSumForVinVout(voutAddresses, vastAddressesWithAmount)
	voutBalanceWithIDs, err = s.bulkBalanceQuery(ctx, voutAddresses...)
	if err != nil {
		return fmt.Errorf("error getting vout balance with ids %v", err)
	}

	for _, voutAddresssWithDeposit := range uVoutAddressesWithSumDeposit {
		isNewBalance := true
		for _, voutBalanceWithID := range voutBalanceWithIDs {
			if voutAddresssWithDeposit.Address == voutBalanceWithID.Balance.Address {
				balance := voutAddresssWithDeposit.Amount.Add(decimal.NewFromFloat(voutBalanceWithID.Balance.Amount))
				amount, _ := balance.Float64()
				updateVoutBalance := elastic.NewBulkUpdateRequest().
					Index(s.network + "-balance").Id(voutBalanceWithID.ID).
					Doc(map[string]interface{}{"amount": amount})
				bulkRequest.Add(updateVoutBalance)
				isNewBalance = false
				break
			}
		}

		if isNewBalance {
			amount, _ := voutAddresssWithDeposit.Amount.Float64()
			newBalance := &domain.Balance{
				Address: voutAddresssWithDeposit.Address,
				Amount:  amount,
			}

			insertBalance := elastic.NewBulkIndexRequest().Index(s.network + "-balance").Doc(newBalance)
			bulkRequest.Add(insertBalance).Refresh("true")
		}
	}

	bulkResponse, err := bulkRequest.Refresh("true").Do(ctx)
	if err != nil {
		return fmt.Errorf("error submiting bulk request for tx %v", err)
	}

	bulkResponse.Created()
	bulkResponse.Updated()
	bulkResponse.Indexed()

	if err := s.bulkInsertBalanceJournalQuery(ctx, vastAddressesWithAmountAndTxID, "sync+"); err != nil {
		return fmt.Errorf("error sync+ journal %v", err)
	}

	if err := s.bulkInsertBalanceJournalQuery(ctx, vinAddressesWithAmountAndTxID, "sync-"); err != nil {
		return fmt.Errorf("error sync- journal %v", err)
	}

	return nil

}

func (s *b2eSync) DumpBlockToEl(ctx context.Context, block *btcjson.GetBlockVerboseResult) error {
	es := s.app.ElasticClient

	if err := s.dumpTxVoutBalance(ctx, block); err != nil {
		return err
	}

	elBlock := domain.BlockWithTxDetail(block)

	_, err := es.Index().Index(s.network + "-block").Id(strconv.FormatInt(block.Height, 10)).
		BodyJson(elBlock).Do(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *b2eSync) createIndexes(ctx context.Context) error {
	es := s.app.ElasticClient

	for _, index := range []string{"block", "tx", "vout", "balance", "balancejournal"} {
		var mapping string
		switch index {
		case "block":
			mapping = data.BlockMapping
		case "tx":
			mapping = data.TxMapping
		case "vout":
			mapping = data.VoutMapping
		case "balance":
			mapping = data.BalanceMapping
		case "balancejournal":
			mapping = data.BalanceJournalMapping
		}

		_, err := es.CreateIndex(s.network + "-" + index).BodyString(mapping).Do(ctx)
		if err != nil {
			s.app.Logger.Warn("Skipping index creation: index exist",
				zap.String("index", s.network+"-"+index))
			continue
		}
	}

	return nil
}

func (s *b2eSync) GetBlockByHeight(height int64) (*btcjson.GetBlockVerboseResult, error) {

	hash, err := s.btcdClient.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	block, err := s.btcdClient.GetBlockVerbose(hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (s *b2eSync) syncBlocksBetween(ctx context.Context, from, to int64) error {
	beginSyncIndex := from - rollback
	if beginSyncIndex <= 0 {
		beginSyncIndex = 1
	}

	for height := beginSyncIndex; height <= to; height++ {
		startSyncTime := time.Now()
		s.app.Logger.Info("Syncing block",
			zap.Int64("height", height),
			zap.Time("time", time.Now()))

		block, err := s.GetBlockByHeight(height)
		if err != nil {
			return err
		}

		for _, txHash := range block.Tx {
			txHashH, err := chainhash.NewHashFromStr(txHash)
			if err != nil {
				return err
			}

			tx, err := s.btcdClient.GetRawTransactionVerbose(txHashH)
			if err != nil {
				return err
			}

			block.RawTx = append(block.RawTx, *tx)
		}

		if height <= (from + int64(rollback+1)) {
			if err := s.rollBackTxVoutBalance(ctx, block); err != nil {
				return err
			}

			es := s.app.ElasticClient
			_, err := es.Delete().Index(s.network + "-block").Id(strconv.FormatInt(height, 10)).
				Refresh("true").Do(ctx)
			if err != nil && err.Error() != "elastic: Error 404 (Not Found)" {
				return err
			}
		}

		if err := s.DumpBlockToEl(ctx, block); err != nil {
			return err
		}

		s.app.Logger.Info("Finished syncing block",
			zap.String("hash", block.Hash),
			zap.Int64("height", height),
			zap.Time("blockTime", time.Unix(block.Time, 0)),
			zap.Time("time", time.Now()),
			zap.Duration("duration", time.Since(startSyncTime)))
	}

	return nil
}

func (s *b2eSync) MaxAgg(ctx context.Context, field, index string) (*float64, error) {
	higestAgg := elastic.NewMaxAggregation().Field(field)
	aggKey := strings.Join([]string{"max", field}, "_")

	searchResult, err := s.app.ElasticClient.Search().
		Index(s.network+"-"+index).Query(elastic.NewMatchAllQuery()).
		Aggregation(aggKey, higestAgg).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	maxAggRes, found := searchResult.Aggregations.Max(aggKey)
	if !found || maxAggRes.Value == nil {
		return nil, errors.New("query max ag error")
	}

	return maxAggRes.Value, nil
}

func (s *b2eSync) DeleteEsTxsByBlockHash(ctx context.Context, blockHash string) error {
	q := elastic.NewTermQuery("blockhash", blockHash)
	if _, err := s.app.ElasticClient.DeleteByQuery().Index(s.network + "-tx").
		Query(q).Refresh("true").Do(ctx); err != nil {
		return err
	}
	return nil
}

func (s *b2eSync) Sync(ctx context.Context) error {

	info, err := s.btcdClient.GetBlockChainInfo()
	if err != nil {
		return err
	}

	var currentHeight float64
	agg, err := s.MaxAgg(ctx, "height", "block")
	if err != nil {
		currentHeight = 1
	} else {
		currentHeight = *agg
	}

	if int64(info.Headers) < int64(currentHeight) {
		return errors.New("synced height is higher than node block high")
	}

	s.app.Logger.Info("Starting from height",
		zap.Float64("current height", currentHeight))

	if err := s.createIndexes(ctx); err != nil {
		return err
	}

	if err := s.syncBlocksBetween(ctx, int64(currentHeight), int64(info.Headers)); err != nil {
		return err
	}

	return nil

}

func (s *b2eSync) queryVoutWithVinsOrVoutsShard(ctx context.Context, utxo []domain.IndexUTXO) ([]domain.VoutWithID, error) {
	var voutWithIDs []domain.VoutWithID

	es := s.app.ElasticClient
	q := elastic.NewBoolQuery()

	for _, vin := range utxo {
		bq := elastic.NewBoolQuery()
		bq.Must(elastic.NewTermQuery("txidbelongto", vin.Txid))
		bq.Must(elastic.NewTermQuery("voutindex", vin.Index))
		q.Should(bq)
	}

	searchResult, err := es.Search().Index(s.network + "-vout").Size(len(utxo)).Query(q).Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, vout := range searchResult.Hits.Hits {
		var elVout domain.VoutStream
		if err := json.Unmarshal(vout.Source, &elVout); err != nil {
			return nil, err
		}
		voutWithIDs = append(voutWithIDs, domain.VoutWithID{
			ID:   vout.Id,
			Vout: &elVout,
		})
	}

	return voutWithIDs, nil
}

func (s *b2eSync) QueryVoutWithVinsOrVouts(ctx context.Context, utxo []domain.IndexUTXO) ([]domain.VoutWithID, error) {
	var utxoTmp []domain.IndexUTXO
	var voutWithIDs []domain.VoutWithID

	for len(utxo) >= 500 {
		utxoTmp, utxo = utxo[:500], utxo[500:]
		voutWithIDsTmp, err := s.queryVoutWithVinsOrVoutsShard(ctx, utxoTmp)
		if err != nil {
			return nil, err
		}

		voutWithIDs = append(voutWithIDs, voutWithIDsTmp...)
	}
	if len(utxo) > 0 {
		voutWithIDsTmp, err := s.queryVoutWithVinsOrVoutsShard(ctx, utxo)
		if err != nil {
			return nil, err
		}

		voutWithIDs = append(voutWithIDs, voutWithIDsTmp...)
	}

	return voutWithIDs, nil
}

func (s *b2eSync) QueryVoutsByUsedFieldAndBelongTxID(ctx context.Context, vins []btcjson.Vin, txBelongto string) ([]domain.VoutWithID, error) {
	if len(vins) == 1 && len(vins[0].Coinbase) != 0 && len(vins[0].Txid) == 0 {
		return nil, errors.New("coinbase tx, vin is new and not exist in es vout Type")
	}
	var esVoutIDS []string

	q := elastic.NewBoolQuery()
	for _, vin := range vins {
		bq := elastic.NewBoolQuery()
		bq = bq.Must(elastic.NewTermQuery("txidbelongto", vin.Txid))
		bq = bq.Must(elastic.NewTermQuery("used.txid", txBelongto))
		bq = bq.Must(elastic.NewTermQuery("used.vinindex", vin.Vout))
		q.Should(bq)
	}

	searchResult, err := s.app.ElasticClient.Search().Index(s.network + "-vout").Size(len(vins)).Query(q).Do(ctx)
	if err != nil {
		return nil, err
	}
	if len(searchResult.Hits.Hits) < 1 {
		return nil, errors.New("vout not found by the condition")
	}

	var voutWithIDs []domain.VoutWithID
	for _, rawHit := range searchResult.Hits.Hits {
		var newVout domain.VoutStream
		if err := json.Unmarshal(rawHit.Source, &newVout); err != nil {
			return nil, err
		}
		esVoutIDS = append(esVoutIDS, rawHit.Id)
		voutWithIDs = append(voutWithIDs, domain.VoutWithID{ID: rawHit.Id, Vout: &newVout})
	}
	return voutWithIDs, nil
}

func NewB2ESync(app *internal.Application, network string) *b2eSync {

	client := app.BtcDTestClient
	if network == "main" {
		client = app.BtcDMainClient
	}

	return &b2eSync{
		network:    network,
		app:        app,
		btcdClient: client,
	}
}
