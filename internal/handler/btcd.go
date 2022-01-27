package handler

import (
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"go.uber.org/zap"
	"sources.witchery.io/coven/btc/internal"
)

type btcdHandler struct {
	app *internal.Application
}

func (h *btcdHandler) onFilteredBlockConnected(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
	h.app.Logger.Info("Filtered block connected",
		zap.Int32("height", height),
		zap.String("hash", header.BlockHash().String()),
		zap.Time("time", header.Timestamp))
}

func (h *btcdHandler) onRescanProgress(hash *chainhash.Hash, height int32, blkTime time.Time) {
	h.app.Logger.Info("Rescan progress",
		zap.Int32("height", height),
		zap.String("hash", hash.String()),
		zap.Time("time", blkTime))
}

func NewBtcDHandler(app *internal.Application) *rpcclient.NotificationHandlers {
	h := &btcdHandler{app: app}

	return &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: h.onFilteredBlockConnected,
		OnRescanProgress:         h.onRescanProgress,
	}
}
