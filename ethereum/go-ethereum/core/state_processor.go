// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"
	"github.com/zhidandeng/collector"
	"github.com/ethereum/go-ethereum/cmd/pluginManage"
	"github.com/ethereum/go-ethereum/dan"
	"github.com/ethereum/go-ethereum/dzd"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts    types.Receipts
		usedGas     = new(uint64)
		header      = block.Header()
		blockHash   = block.Hash()
		blockNumber = block.Number()
		allLogs     []*types.Log
		gp          = new(GasPool).AddGas(block.GasLimit())
	)
	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
	//add
	if p.config.TransferDataPlg.GetOpcodeRegister("handle_BLOCK_INFO") {
		blockcollector := collector.NewBlockCollector()
		blockcollector.Op = "Block" + fmt.Sprintf("%v", header.Number)
		blockcollector.ParentHash = header.ParentHash.String()
		blockcollector.UncleHash = header.UncleHash.String()
		blockcollector.Coinbase = header.Coinbase.String()
		blockcollector.StateRoot = header.Root.String()
		blockcollector.TxHashRoot = header.TxHash.String()
		blockcollector.ReceiptHash = header.ReceiptHash.String()
		blockcollector.Difficulty = header.Difficulty.String()
		blockcollector.Number = header.Number.String()
		blockcollector.GasLimit = header.GasLimit
		blockcollector.GasUsed = header.GasUsed
		blockcollector.Time = header.Time
		blockcollector.Extra = header.Extra
		blockcollector.MixDigest = header.MixDigest.String()
		blockcollector.Nonce = header.Nonce.Uint64()
		p.config.TransferDataPlg.SendDataToPlugin("handle_BLOCK_INFO", blockcollector.SendBlockInfo("handle_BLOCK_INFO"))
	}
	//add
	blockContext := NewEVMBlockContext(header, p.bc, nil)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, p.config, cfg)
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		msg, err := tx.AsMessage(types.MakeSigner(p.config, header.Number), header.BaseFee)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		statedb.Prepare(tx.Hash(), i)
		receipt, err := applyTransaction(msg, p.config, nil, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles())

	return receipts, allLogs, *usedGas, nil
}

func applyTransaction(msg types.Message, config *params.ChainConfig, author *common.Address, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM) (*types.Receipt, error) {
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	//add

	if dzd.BLOCKING_FLAG == true {
		statedb.RevertToSnapshot(dzd.PLUGIN_SNAPSHOT_ID)
	}
	tcend := collector.NewTransCollector()

	vmenv := evm
	if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOEND") {
		tcend.Op = "EXTERNALINFOEND"
		tcend.TxHash = tx.Hash().String()
		tcend.GasUsed = result.UsedGas
		tcend.CallLayer = 1
	}
	//add

	if err != nil {
		//add
		if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOEND") {
			tcend.IsSuccess = false
			vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("EXTERNALINFOEND", tcend.SendTransInfo("EXTERNALINFOEND"))
		}
		//add
		return nil, err
	}

	// Update the state with pending changes.
	var root []byte
	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
		//add
		if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOEND") {
			tcend.CallType = "CREATE"
			tcend.To = receipt.ContractAddress.String()
			createcollector := collector.NewCreateCollector()
			createcollector.ContractAddr = receipt.ContractAddress.String()
			createcollector.ContractDeployCode = msg.Data()
			if vmenv.StateDB.Exist(receipt.ContractAddress) {
				createcollector.ContractRuntimeCode = vmenv.StateDB.GetCode(receipt.ContractAddress)
			}
			tcend.CreateInfo = *createcollector
		}
		//add
	}

	// Set the receipt logs and create the bloom filter.
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())

	//add
	if !result.Failed() {
		if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOEND") {
			tcend.IsSuccess = true
			vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("EXTERNALINFOEND", tcend.SendTransInfo("EXTERNALINFOEND"))
		}
	} else {
		if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOEND") {
			tcend.IsSuccess = false
			vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("EXTERNALINFOEND", tcend.SendTransInfo("EXTERNALINFOEND"))
		}
	}

	dzd.CALL_STACK = dzd.CALL_STACK[:len(dzd.CALL_STACK)-1]

	if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("TXEND") {
		vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("TXEND", collector.SendFlag("TXEND"))
		vmenv.ChainConfig().TransferDataPlg.Stop()
	}
	//add
	return receipt, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number), header.BaseFee)
	if err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, cfg)

	//add
	vmenv.SetTxStart(true)
	vmenv.ChainConfig().TransferDataPlg.Start()

	if dan.IsReg {
		// //whole folder fresh有问题，如果全部移除，是无法删掉旧的的。需要用new去新增
		// pluginManage.StartRun(vmenv.ChainConfig().TransferDataPlg)
		// dan.IsReg = false

		//single plugin
		pluginManage.RegisterPlugin(vmenv.ChainConfig().TransferDataPlg, dan.RegPath)
		dan.RegPath = dan.Clear
		dan.IsReg = false
	}

	if dan.IsUn {
		vmenv.ChainConfig().TransferDataPlg.UnRegisterPlg()
		dan.IsUn = false
		dan.UnPlg = dan.Clear
	}

	dzd.CALL_LAYER = 0
	dzd.CALL_STACK = nil
	dzd.ALL_STACK = nil
	dzd.EXTERNAL_FLAG = true
	dzd.BLOCKING_FLAG = false
	dzd.PLUGIN_SNAPSHOT_ID = 0
	dzd.CALLVALID_MAP = make(map[int]bool)
	dzd.TxHash = tx.Hash().String()

	if msg.To() != nil {
		dzd.CALL_LAYER += 1
		dzd.CALL_STACK = append(dzd.CALL_STACK, msg.To().String()+"#"+strconv.Itoa(dzd.CALL_LAYER))
		dzd.ALL_STACK = append(dzd.ALL_STACK, msg.To().String())
	}

	if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("TXSTART") {
		vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("TXSTART", collector.SendFlag("TXSTART"))
	}

	tcstart := collector.NewTransCollector()

	//external collector
	if vmenv.ChainConfig().TransferDataPlg.GetOpcodeRegister("EXTERNALINFOSTART") {
		tcstart.Op = "EXTERNALINFOSTART"
		tcstart.TxHash = tx.Hash().String()
		tcstart.BlockNumber = blockContext.BlockNumber.String()
		tcstart.BlockTime = blockContext.Time.String()
		tcstart.From = msg.From().String()
		tcstart.Value = msg.Value().String()
		tcstart.GasPrice = msg.GasPrice().String()
		tcstart.GasLimit = msg.Gas()
		tcstart.Nonce = tx.Nonce()
		tcstart.CallLayer = 1
		if msg.To() != nil {
			tcstart.CallType = "CALL"
			tcstart.To = msg.To().String()

			callcollector := collector.NewCallCollector()
			if vmenv.StateDB.Exist(*msg.To()) {
				callcollector.ContractCode = vmenv.StateDB.GetCode(*msg.To())
			}
			callcollector.InputData = msg.Data()
			tcstart.CallInfo = *callcollector
		}
		vmenv.ChainConfig().TransferDataPlg.SendDataToPlugin("EXTERNALINFOSTART", tcstart.SendTransInfo("EXTERNALINFOSTART"))

	}
	//add

	return applyTransaction(msg, config, author, gp, statedb, header.Number, header.Hash(), tx, usedGas, vmenv)
}
