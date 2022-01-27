package domain

import (
	"errors"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/shopspring/decimal"
)

type Balance struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

type BalanceJournal struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
	Operate string  `json:"operate"`
	Txid    string  `json:"txid"`
}

type AddressWithAmount struct {
	Address string          `json:"address"`
	Amount  decimal.Decimal `json:"amount"`
}

type AddressWithAmountAndTxid struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
	Txid    string  `json:"txid"`
}

type BalanceWithID struct {
	ID      string  `json:"id"`
	Balance Balance `json:"balance"`
}

type VoutWithID struct {
	ID   string
	Vout *VoutStream
}

type VoutStream struct {
	TxIDBelongTo string      `json:"txidbelongto"`
	Value        float64     `json:"value"`
	Voutindex    uint32      `json:"voutindex"`
	Coinbase     bool        `json:"coinbase"`
	Addresses    []string    `json:"addresses"`
	Used         interface{} `json:"used"`
}

type AddressWithValueInTx struct {
	Address string  `json:"address"`
	Value   float64 `json:"value"`
}

type IndexUTXO struct {
	Txid  string
	Index uint32
}

type esTx struct {
	Txid      string                 `json:"txid"`
	Fee       float64                `json:"fee"`
	BlockHash string                 `json:"blockhash"`
	Time      int64                  `json:"time"`
	Vins      []AddressWithValueInTx `json:"vins"`
	Vouts     []AddressWithValueInTx `json:"vouts"`
}

type VoutUsed struct {
	Txid     string `json:"txid"`
	VinIndex uint32 `json:"vinindex"`
}

func BlockWithTxDetail(block *btcjson.GetBlockVerboseResult) interface{} {
	txs := BlockTx(block.RawTx)
	blockWithTx := map[string]interface{}{
		"hash":         block.Hash,
		"strippedsize": block.StrippedSize,
		"size":         block.Size,
		"weight":       block.Weight,
		"height":       block.Height,
		"versionHex":   block.VersionHex,
		"merkleroot":   block.MerkleRoot,
		"time":         block.Time,
		"nonce":        block.Nonce,
		"bits":         block.Bits,
		"difficulty":   block.Difficulty,
		"previoushash": block.PreviousHash,
		"nexthash":     block.NextHash,
		"tx":           txs,
	}
	return blockWithTx
}

func BlockTx(txs []btcjson.TxRawResult) []map[string]interface{} {
	var rawTxs []map[string]interface{}
	for _, tx := range txs {
		// https://tradeblock.com/blog/bitcoin-0-8-5-released-provides-critical-bug-fixes/
		txVersion := tx.Version
		if tx.Version < 0 {
			txVersion = 1
		}

		if tx.Version >= 32767 {
			txVersion = 32767
		}
		vouts := TxVouts(tx)
		vins := TxVins(tx)
		rawTxs = append(rawTxs, map[string]interface{}{
			"txid":     tx.Txid,
			"hash":     tx.Hash,
			"version":  txVersion,
			"size":     tx.Size,
			"vsize":    tx.Vsize,
			"locktime": tx.LockTime,
			"vout":     vouts,
			"vin":      vins,
		})
	}
	return rawTxs
}

func TxVouts(tx btcjson.TxRawResult) []map[string]interface{} {
	var vouts []map[string]interface{}
	for _, vout := range tx.Vout {
		vouts = append(vouts, map[string]interface{}{
			"value": vout.Value,
			"n":     vout.N,
			"scriptPubKey": map[string]interface{}{
				"asm":       vout.ScriptPubKey.Asm,
				"reqSigs":   vout.ScriptPubKey.ReqSigs,
				"type":      vout.ScriptPubKey.Type,
				"addresses": vout.ScriptPubKey.Addresses,
			},
		})
	}
	return vouts
}

func TxVins(tx btcjson.TxRawResult) []map[string]interface{} {
	var vins []map[string]interface{}
	for _, vin := range tx.Vin {
		if len(tx.Vin) == 1 && len(vin.Coinbase) != 0 && len(vin.Txid) == 0 {
			vins = append(vins, map[string]interface{}{
				"coinbase": vin.Coinbase,
				"sequence": vin.Sequence,
			})
			break
		}
		vins = append(vins, map[string]interface{}{
			"txid": vin.Txid,
			"vout": vin.Vout,
			"scriptSig": map[string]interface{}{
				"asm": vin.ScriptSig.Asm,
			},
			"sequence": vin.Sequence,
		})
	}
	return vins
}

func NewVout(vout btcjson.Vout, vins []btcjson.Vin, TxID string) (*VoutStream, error) {
	coinbase := false
	if len(vins[0].Coinbase) != 0 && len(vins[0].Txid) == 0 {
		coinbase = true
	}

	addresses, err := VoutAddress(vout)
	if err != nil {
		return nil, err
	}

	v := &VoutStream{
		TxIDBelongTo: TxID,
		Value:        vout.Value,
		Voutindex:    vout.N,
		Coinbase:     coinbase,
		Addresses:    *addresses,
		Used:         nil,
	}
	return v, nil
}

func NewElTx(tx btcjson.TxRawResult, fee float64, blockHash string, simpleVins, simpleVouts []AddressWithValueInTx) *esTx {
	result := &esTx{
		Txid:      tx.Txid,
		Fee:       fee,
		BlockHash: blockHash,
		Time:      tx.Time,
		Vins:      simpleVins,
		Vouts:     simpleVouts,
	}
	return result
}

func NewBalanceJournalFun(address, ope, txid string, amount float64) BalanceJournal {
	return BalanceJournal{
		Address: address,
		Operate: ope,
		Amount:  amount,
		Txid:    txid,
	}
}

func VoutAddress(vout btcjson.Vout) (*[]string, error) {
	if len(vout.ScriptPubKey.Addresses) > 0 {
		return &vout.ScriptPubKey.Addresses, nil
	}

	return nil, errors.New("unable to decode output address")
}
