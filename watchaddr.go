// Watch for transactions receiving to or sending from certain addresses.
// Receiving works, sending is probably messed up.
package main

import (
	"fmt"
	"sync"

	_ "github.com/decred/dcrd/chaincfg/chainhash"
	_ "github.com/decred/dcrd/dcrjson"
	"github.com/decred/dcrd/txscript"
	"github.com/decred/dcrrpcclient"
	"github.com/decred/dcrutil"
)

func handleReceivingTx(c *dcrrpcclient.Client, addrs map[string]struct{},
	recvTxChan <-chan *watchedAddrTx, wg *sync.WaitGroup,
	quit <-chan struct{}) {
	defer wg.Done()
	//out:
	for {
		//keepon:
		select {
		case addrTx, ok := <-recvTxChan:
			if !ok {
				log.Infof("Receive Tx watch channel closed")
				return
			}

			// Make like notifyForTxOuts and screen the transactions TxOuts for
			// addresses we are watching for.

			tx := addrTx.transaction
			var action string
			if addrTx.details != nil {
				action = fmt.Sprintf("mined into block %d.", addrTx.details.Height)
			} else {
				action = "inserted into mempool."
			}

			for _, txOut := range tx.MsgTx().TxOut {
				_, txAddrs, _, err := txscript.ExtractPkScriptAddrs(txOut.Version,
					txOut.PkScript, activeChain)
				if err != nil {
					log.Infof("ExtractPkScriptAddrs: %v", err.Error())
					continue
				}

				for _, txAddr := range txAddrs {
					addrstr := txAddr.EncodeAddress()
					if _, ok := addrs[addrstr]; ok {
						log.Infof("Transaction with watched address %v as outpoint (receiving), value %.6f, %v",
							addrstr, dcrutil.Amount(txOut.Value).ToCoin(),
							action)
						continue
					}
				}
			}

		case <-quit:
			mempoolLog.Debugf("Quitting OnRecvTx/OnRedeemingTx handler.")
			return
		}
	}

}

// Rather than watching for the sending address, which isn't known ahead of
// time, watch for a transaction with an input (source) whos previous outpoint
// is one of the watched addresses.
// But I am not sure we can do that here with the Tx and BlockDetails provided.
func handleSendingTx(c *dcrrpcclient.Client, addrs map[string]struct{},
	spendTxChan <-chan *watchedAddrTx, wg *sync.WaitGroup,
	quit <-chan struct{}) {
	defer wg.Done()
	//out:
	for {
		//keepon:
		select {
		case addrTx, ok := <-spendTxChan:
			if !ok {
				log.Infof("Send Tx watch channel closed")
				return
			}

			// Unfortunately, can't make like notifyForTxOuts because we are
			// not watching outpoints.  For the tx we are given, we need to
			// search through each TxIn's PreviousOutPoints, requesting the raw
			// transaction from each PreviousOutPoint's tx hash, and check each
			// TxOut in the result for each watched address.  Phew! There is
			// surely a better way, but I don't know it.

			tx := addrTx.transaction
			var action string
			if addrTx.details != nil {
				action = fmt.Sprintf("mined into block %d.", addrTx.details.Height)
			} else {
				action = "inserted into mempool."
			}

			for _, txIn := range tx.MsgTx().TxIn {
				prevOut := &txIn.PreviousOutPoint
				// uh, now what?
				// For each TxIn, we need to check the indicated vout index in the
				// txid of the previous outpoint.
				//txrr, err := c.GetRawTransactionVerbose(&prevOut.Hash)
				Tx, err := c.GetRawTransaction(&prevOut.Hash)
				if err != nil {
					log.Error("Unable to get raw transaction for", Tx)
				}

				// prevOut.Index should tell us which one, right?  Check all anyway.
				for _, txOut := range Tx.MsgTx().TxOut {
					_, txAddrs, _, err := txscript.ExtractPkScriptAddrs(
						txOut.Version, txOut.PkScript, activeChain)
					if err != nil {
						log.Infof("ExtractPkScriptAddrs: %v", err.Error())
						continue
					}

					for _, txAddr := range txAddrs {
						addrstr := txAddr.EncodeAddress()
						if _, ok := addrs[addrstr]; ok {
							log.Infof("Transaction with watched address %v as previous outpoint (spending), value %.6f, %v",
								addrstr, dcrutil.Amount(txOut.Value).ToCoin(), action)
							continue
						}
					}
				}

				// That's not what I'm doing here, but I'm looking anyway...
				// log.Debug(txscript.GetScriptClass(txscript.DefaultScriptVersion, txIn.SignatureScript))
				// log.Debug(txscript.GetPkScriptFromP2SHSigScript(txIn.SignatureScript))
				// sclass, txAddrs, nreqsigs, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, txIn.SignatureScript, activeChain)
				// log.Debug(sclass, txAddrs, nreqsigs, err, action)

				// addresses := make([]string, len(txAddrs))
				// for i, addr := range txAddrs {
				// 	addresses[i] = addr.EncodeAddress()
				// }
				// log.Debug(addresses)
			}
		case <-quit:
			mempoolLog.Debugf("Quitting OnRecvTx/OnRedeemingTx handler.")
			return
		}
	}
}