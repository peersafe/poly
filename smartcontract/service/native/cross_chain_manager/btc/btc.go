/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package btc

import (
	"bytes"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	wire_bch "github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/merkleblock"
	"github.com/ontio/multi-chain/smartcontract/service/native"
)

const (
	BTC_TX_PREFIX string = "btctx"
)

// Verify merkle proof in bytes, and return the result in true or false
// Firstly, calculate the merkleRoot from input `proof`; Then get header.MerkleRoot
// by a spv client and check if they are equal.
func VerifyBtcTx(proof []byte, tx []byte, height uint32) (bool, error) {
	mb := wire_bch.MsgMerkleBlock{}
	err := mb.BchDecode(bytes.NewReader(proof), wire_bch.ProtocolVersion, wire_bch.LatestEncoding)
	if err != nil {
		return false, err
	}

	mtx := wire.NewMsgTx(wire.TxVersion)
	reader := bytes.NewReader(tx)
	err = mtx.BtcDecode(reader, wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		return false, fmt.Errorf("VerifyBtc, failed to decode the transaction")
	}
	txid := mtx.TxHash()
	if !bytes.Equal(mb.Hashes[0][:], txid[:]) && !bytes.Equal(mb.Hashes[1][:], txid[:]) {
		return false, fmt.Errorf("VerifyBtc, wrong transaction hash: %s in proof are not equal with %s",
			mb.Hashes[0].String(), txid.String())
	}

	mBlock := merkleblock.NewMerkleBlockFromMsg(mb)
	merkleRootCalc := mBlock.ExtractMatches()
	if merkleRootCalc == nil || mBlock.BadTree() || len(mBlock.GetMatches()) == 0 {
		return false, fmt.Errorf("VerifyBtc, bad merkle tree")
	}

	header, err := NewRestClient().GetHeaderFromSpv(height)
	if err != nil {
		return false, fmt.Errorf("VerifyBtc, failed to get header from spv client: %v", err)
	}

	if !bytes.Equal(merkleRootCalc[:], header.MerkleRoot[:]) {
		return false, fmt.Errorf("VerifyBtc, merkle root not equal")
	}

	return true, nil
}

// Create a raw transaction that returns the BTC that once locked the multi-sign account
// to the original account and this transacion is not signed. In the end of this function,
// serialized raw transaction would be put into native.CacheDB.
// Parameter `prevTxids` is the txid of the previous output of the transaction input reference,
// `prevIndexes` contain the indexes of the output in the transaction, `amounts` is the mapping
// of accounts and amounts in transaction's output. Return true if building transacion success.
func MakeBtcTx(native *native.NativeService, prevTxids []string, prevIndexes []uint32, amounts map[string]float64) (bool, error) {
	if len(prevIndexes) != len(prevTxids) || len(prevTxids) == 0 {
		return false, fmt.Errorf("MakeBtcTx, wrong num of transaction's inputs")
	}
	var txIns []btcjson.TransactionInput
	for i := 0; i < len(prevTxids); i++ {
		txIns = append(txIns, btcjson.TransactionInput{
			Txid: prevTxids[i],
			Vout: prevIndexes[i],
		})
	}

	mtx, err := getRawTx(txIns, amounts, nil)
	if err != nil {
		return false, fmt.Errorf("MakeBtcTx, get rawtransaction fail: %v", err)
	}
	var buf bytes.Buffer
	err = mtx.BtcEncode(&buf, wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		return false, fmt.Errorf("MakeBtcTx, serialize rawtransaction fail: %v", err)
	}

	native.CacheDB.Put([]byte(BTC_TX_PREFIX+"btctx??"), buf.Bytes())
	return true, nil
}

// This function needs to input the input and output information of the transaction
// and the lock time. Function build a raw transaction without signature and return it.
// This function uses the partial logic and code of btcd to finally return the
// reference of the transaction object.
func getRawTx(txIns []btcjson.TransactionInput, amounts map[string]float64, locktime *int64) (*wire.MsgTx, error) {
	if locktime != nil &&
		(*locktime < 0 || *locktime > int64(wire.MaxTxInSequenceNum)) {
		return nil, fmt.Errorf("getRawTx, locktime %d out of range", *locktime)
	}

	// Add all transaction inputs to a new transaction after performing
	// some validity checks.
	mtx := wire.NewMsgTx(wire.TxVersion)
	for _, input := range txIns {
		txHash, err := chainhash.NewHashFromStr(input.Txid)
		if err != nil {
			return nil, fmt.Errorf("getRawTx, decode txid fail: %v", err)
		}

		prevOut := wire.NewOutPoint(txHash, input.Vout)
		txIn := wire.NewTxIn(prevOut, []byte{}, nil)
		if locktime != nil && *locktime != 0 {
			txIn.Sequence = wire.MaxTxInSequenceNum - 1
		}
		mtx.AddTxIn(txIn)
	}

	// Add all transaction outputs to the transaction after performing
	// some validity checks.
	params := &chaincfg.MainNetParams
	for encodedAddr, amount := range amounts {
		// Ensure amount is in the valid range for monetary amounts.
		if amount <= 0 || amount > btcutil.MaxSatoshi {
			return nil, fmt.Errorf("getRawTx, wrong amount: %f", amount)
		}

		// Decode the provided address.
		addr, err := btcutil.DecodeAddress(encodedAddr, params)
		if err != nil {
			return nil, fmt.Errorf("getRawTx, decode addr fail: %v", err)
		}

		// Ensure the address is one of the supported types and that
		// the network encoded with the address matches the network the
		// server is currently on.
		switch addr.(type) {
		case *btcutil.AddressPubKeyHash:
		default:
			return nil, fmt.Errorf("getRawTx, type of addr is not found")
		}
		if !addr.IsForNet(params) {
			return nil, fmt.Errorf("getRawTx, addr is not for mainnet")
		}

		// Create a new script which pays to the provided address.
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, fmt.Errorf("getRawTx, failed to generate pay-to-address script: %v", err)
		}

		// Convert the amount to satoshi.
		satoshi, err := btcutil.NewAmount(amount)
		if err != nil {
			return nil, fmt.Errorf("getRawTx, failed to convert amount: %v", err)
		}

		txOut := wire.NewTxOut(int64(satoshi), pkScript)
		mtx.AddTxOut(txOut)
	}

	// Set the Locktime, if given.
	if locktime != nil {
		mtx.LockTime = uint32(*locktime)
	}

	return mtx, nil
}