package util

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/shopspring/decimal"
	"sources.witchery.io/coven/btc/internal/domain"
)

func ParseESVout(voutWithID domain.VoutWithID, txid string) ([]domain.AddressWithValueInTx, []interface{},
	[]domain.Balance, []domain.AddressWithAmountAndTxid) {
	var (
		txTypeVinsField                  []domain.AddressWithValueInTx
		vinAddresses                     []interface{}
		vinAddressWithAmountSlice        []domain.Balance
		vinAddressWithAmountAndTxidSlice []domain.AddressWithAmountAndTxid
	)

	for _, address := range voutWithID.Vout.Addresses {
		vinAddresses = append(vinAddresses, address)
		vinAddressWithAmountSlice = append(vinAddressWithAmountSlice,
			domain.Balance{Address: address, Amount: voutWithID.Vout.Value})
		txTypeVinsField = append(txTypeVinsField,
			domain.AddressWithValueInTx{Address: address, Value: voutWithID.Vout.Value})
		vinAddressWithAmountAndTxidSlice = append(vinAddressWithAmountAndTxidSlice, domain.AddressWithAmountAndTxid{
			Address: address, Amount: voutWithID.Vout.Value, Txid: txid})
	}
	return txTypeVinsField, vinAddresses, vinAddressWithAmountSlice, vinAddressWithAmountAndTxidSlice
}

func IndexedVouts(vouts []btcjson.Vout, txid string) []domain.IndexUTXO {
	var IndexUTXOs []domain.IndexUTXO
	for _, vout := range vouts {
		IndexUTXOs = append(IndexUTXOs, domain.IndexUTXO{
			Txid:  txid,
			Index: vout.N,
		})
	}
	return IndexUTXOs
}

func IndexVins(vins []btcjson.Vin) []domain.IndexUTXO {
	var index []domain.IndexUTXO
	for _, vin := range vins {
		index = append(index, domain.IndexUTXO{
			Txid:  vin.Txid,
			Index: vin.Vout,
		})
	}
	return index
}

func CalcUniqueAddressSumForVinVout(addresses []interface{}, balances []domain.Balance) []*domain.AddressWithAmount {
	uniqueAddresses := RemoveSliceDuplicates(addresses...)
	uniqueAddressesWithSum := make([]*domain.AddressWithAmount, len(uniqueAddresses))

	for i, a := range uniqueAddresses {
		deposit := decimal.NewFromFloat(0)
		for _, b := range balances {
			if a == b.Address {
				deposit = deposit.Add(decimal.NewFromFloat(b.Amount))
			}
		}
		uniqueAddressesWithSum[i] = &domain.AddressWithAmount{
			Address: a,
			Amount:  deposit,
		}
	}

	return uniqueAddressesWithSum
}

func RemoveSliceDuplicates(slice ...interface{}) []string {
	encounter := make(map[string]bool)
	for _, v := range slice {
		encounter[v.(string)] = true
	}

	index := 0
	result := make([]string, len(encounter))
	for key := range encounter {
		result[index] = key
		index++
	}

	return result
}
