package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/filechain/filechain/btcec"
	"github.com/filechain/filechain/btcjson"
	"github.com/filechain/filechain/btcutil"
	"github.com/filechain/filechain/btcwallet/wallet/txrules"
	"github.com/filechain/filechain/chaincfg/chainhash"
	"github.com/filechain/filechain/database"
	"github.com/filechain/filechain/txscript"
	"github.com/filechain/filechain/wire"
)

const CommunityAddr = "STzkcq92qQigxd5qEeZrTkd4s3MRdRjxMr"

var fileChainHashBucketName = []byte("filechainhashidx")
var fileChainBuyHashBucketName = []byte("filechainbuyhashidx")

var chain *BlockChain

func GetBuyHashInfo(cmd *btcjson.GetBuyHashCmd) (*btcjson.BuyHashInfoRsp, error) {
	var result *btcjson.BuyHashInfoRsp
	hash, err := hex.DecodeString(cmd.Hash)
	if err != nil {
		return nil, err
	}
	if err := chain.db.View(func(dbTx database.Tx) error {
		info, err := dbFetchBuyInfoByHash(dbTx, hash)
		if err != nil {
			return err
		}
		result = &btcjson.BuyHashInfoRsp{
			BlockHeight:      info.BlockHeight,
			BuyHash:          hex.EncodeToString(info.BuyHash),
			FileHash:         hex.EncodeToString(info.FileHash),
			Buyer:            string(info.Buyer),
			TransFee:         btcutil.Amount(info.TransFee).ToBTC(),
			TransFeeTx:       hex.EncodeToString(info.TransFeeTx),
			TransFeeVoutIdx:  info.TransFeeVoutIdx,
			DepositValueLeft: btcutil.Amount(info.DepositValueLeft).ToBTC(),
			FinalTransFees:   make(map[string]float64),
		}
		for _, fee := range info.FinalTransFees {
			result.FinalTransFees[string(fee.Account)] = btcutil.Amount(fee.Fee).ToBTC()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func ListSingleHash(cmd *btcjson.ListSingleHashCmd) (*btcjson.FileHashInfoRsp, error) {
	var result *btcjson.FileHashInfoRsp
	hash, err := hex.DecodeString(cmd.Hash)
	if err != nil {
		return nil, err
	}
	if err := chain.db.View(func(dbTx database.Tx) error {
		info, err := dbFetchFileInfoByHash(dbTx, hash)
		if err != nil {
			return err
		}
		result = &btcjson.FileHashInfoRsp{
			BlockHeight:   info.BlockHeight,
			Hash:          hex.EncodeToString(info.Hash),
			Title:         string(info.Title),
			Price:         btcutil.Amount(info.Price).ToBTC(),
			Author:        string(info.Author),
			IsSexually:    info.FileTag&(1<<btcjson.Sexually) == 1<<btcjson.Sexually,
			IsPolitically: info.FileTag&(1<<btcjson.Politically) == 1<<btcjson.Politically,
			IsOriginal:    info.FileTag&(1<<btcjson.Original) == 1<<btcjson.Original,
			IsViolence:    info.FileTag&(1<<btcjson.Violence) == 1<<btcjson.Violence,
			DownloadSucc:  info.DownloadSucc,
			DownloadFail:  info.DownloadFail,
			LastSuccTime:  info.LastSuccTime,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func ListHash(cmd *btcjson.ListHashCmd) (*btcjson.ListHashResult, error) {
	var idx int64
	infos := make([]*btcjson.FileHashInfoRsp, 0, *cmd.Limit)
	if err := chain.db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		bucket := meta.Bucket(fileChainHashBucketName)
		cursor := bucket.Cursor()
		skip := 0
		if cursor.Last() {
			cursorValid := true
			for int64(skip) < *cmd.Offset {
				skip++
				if !cursor.Prev() {
					cursorValid = false
					break
				}
			}
			if cursorValid && int64(skip) >= *cmd.Offset {
				for len(infos) < int(*cmd.Limit) {
					var info = &btcjson.FileHashInfo{}
					info.FromBytes(cursor.Value())
					infos = append(infos, &btcjson.FileHashInfoRsp{
						BlockHeight:   info.BlockHeight,
						Hash:          hex.EncodeToString(info.Hash),
						Title:         string(info.Title),
						Price:         btcutil.Amount(info.Price).ToBTC(),
						Author:        string(info.Author),
						IsSexually:    info.FileTag&(1<<btcjson.Sexually) == 1<<btcjson.Sexually,
						IsPolitically: info.FileTag&(1<<btcjson.Politically) == 1<<btcjson.Politically,
						IsOriginal:    info.FileTag&(1<<btcjson.Original) == 1<<btcjson.Original,
						IsViolence:    info.FileTag&(1<<btcjson.Violence) == 1<<btcjson.Violence,
						DownloadSucc:  info.DownloadSucc,
						DownloadFail:  info.DownloadFail,
						LastSuccTime:  info.LastSuccTime,
					})
					if !cursor.Prev() {
						break
					}
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result := &btcjson.ListHashResult{
		Total:  idx,
		Result: infos,
	}

	return result, nil
}

type OPCodePOS uint8

const (
	OPP_TxOut OPCodePOS = iota
	OPP_TxIn
	OPP_TxPrevOut
)

// 一个检查的执行点，如果是本事务的一个 txout，则 相应的txin 为空，如果是prevOut，则txin为对应的txin
type ExecutePoint struct {
	pos   OPCodePOS
	txin  *wire.TxIn
	txout *wire.TxOut
}

type FCEExecute func(uint8, []interface{}, OPCodePOS, *btcjson.FileHashInfo, *btcjson.BuyHashInfo, database.Tx, *btcutil.Block, *btcutil.Tx, ExecutePoint) (bool, bool, error)

// 当一个事务被递交到全节点的时候，节点根据制定的规则共识进行文件链相关的检查
// 如果有节点多次广播违反共识的事务，将被拒绝连接
var FCERejectRules = []FCEExecute{
	func(code uint8, params []interface{}, pos OPCodePOS, paramcheckfileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if pos == OPP_TxOut && code == txscript.OP_FILE_PUBLISH {
			lastshare, _ := dbFetchFileInfoByHash(dbTx, params[0].([]byte))
			if lastshare != nil {
				return false, false, fmt.Errorf("已经分享过的文件Hash不允许二次分享")
			}
			paramcheck := btcjson.FileHashInfo{}
			paramcheck.Price = params[1].(int64)
			if paramcheck.Price <= 0 {
				return false, false, fmt.Errorf("不允许分享价格小于等于0的内容")
			}
			paramcheck.Title = params[2].([]byte)
			if len(paramcheck.Title) < 5 {
				return false, false, fmt.Errorf("不允许标题少于5个字符的内容")

			}
			paramcheck.Author = params[3].([]byte)
			if _, err := btcutil.DecodeAddress(string(paramcheck.Author), chain.chainParams); err != nil {
				return false, false, err
			}
		}
		return false, false, nil
	},
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if pos == OPP_TxOut && code == txscript.OP_FILE_BUY_PAY {
			var err error
			txout := ep.txout
			if fileinfo == nil {
				filehash := params[1].([]byte)

				fileinfo, err = dbFetchFileInfoByHash(dbTx, filehash)
				if err != nil {
					return false, false, err
				}
				if fileinfo == nil {
					return false, false, fmt.Errorf("购买时候找不到文件信息")
				}
			}
			if fileinfo.Price<<1 > txout.Value {
				return false, false, fmt.Errorf("购买押金需要大于两倍文件价格: %f < %f", btcutil.Amount(fileinfo.Price).ToBTC(), btcutil.Amount(txout.Value).ToBTC())
			}
		}
		return false, false, nil
	},
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if pos == OPP_TxOut && code == txscript.OP_FILE_CONFIRM_BUY {
			txout := ep.txout
			if fileinfo.Price > txout.Value {
				return false, false, fmt.Errorf("支付金额(%f)要大于等于文件价格(%f)", btcutil.Amount(fileinfo.Price).ToBTC(), btcutil.Amount(txout.Value).ToBTC())
			}
		}
		return false, false, nil
	},
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		// 花费各种押金的规则
		if pos == OPP_TxPrevOut && code == txscript.OP_FILE_PUBLISH {
			//花费发布押金的规则
			filehash := params[0].([]byte)
			fileinfo, err := dbFetchFileInfoByHash(dbTx, filehash)
			if err != nil {
				return false, false, err
			}
			if fileinfo.DownloadSucc < txrules.DefaultDepositTimesOfDownload {
				return false, false, fmt.Errorf("未成功下载超过 %d 规定次数的文件押金不许取回", txrules.DefaultDepositTimesOfDownload)
			}
		}
		if pos == OPP_TxPrevOut && code == txscript.OP_FILE_BUY_PAY {
			// 购买押金的花费规则，购买押金只可以用户支持确认购买信息
			// 也就是说 事务中所有的txout都必须是确认购买并且是本次购买的确认
			total := int64(0)
			totalBuyTrans := 0
			log.Infof("花费FileBuyPay，检查花到什么地方去，共计检查 %d 个txout", len(tx.MsgTx().TxOut))
			for _, txout := range tx.MsgTx().TxOut {
				fcparams, err := txscript.ExtractFCEOperation(txout.PkScript)
				if err != nil {
					return false, false, err
				}
				if fcparams == nil || (fcparams.Code != txscript.OP_FILE_CONFIRM_BUY && fcparams.Code != txscript.OP_FILE_CONFIRM_TRANSFEE && fcparams.Code != txscript.OP_FILE_BUY_TRANS) {
					return false, false, fmt.Errorf("购买押金只能用于支付确认购买所需费用")
				}
				if !bytes.Equal(fcparams.Params[0].([]byte), buyinfo.BuyHash) {
					return false, false, fmt.Errorf("购买押金与用于支付确认费用的购买行为必须一致")
				}
				if fcparams.Code != txscript.OP_FILE_BUY_TRANS {
					total += txout.Value
				} else {
					totalBuyTrans++
					if totalBuyTrans > 1 {
						return false, false, fmt.Errorf("花费购买押金时候只允许转移 1 笔资金到传输押金")
					}
				}
			}
			if total > buyinfo.TransFee+fileinfo.Price {
				return false, false, fmt.Errorf("用于支付确认费用的总金额不得高于传输费和文件代价的总和")
			}
		}
		if pos == OPP_TxPrevOut && code == txscript.OP_FILE_BUY_TRANS {
			//花费购买的时候传输费押金的规则，用户确认购买以后，剩余的押金会转移成传输押金
			//传输押金在两种情况下可能被消费，一种是100个块内没有传输者提出异议，这种情况下只能由购买者花费
			//一种情况是有传输者提出证据证明购买者有传输费作弊行为，这种情况下可以由提出证据的人花费
			if len(buyinfo.FinalTransFees) == 0 {
				return false, false, fmt.Errorf("没有完成下载，不允许取花费输押金")
			}

			// _, depositAddr, _, _, _, err := txscript.ExtractPkScriptAddrs(ep.txout.PkScript, chain.chainParams)
			// if err != nil {
			// 	return false, false, err
			// }
			// if len(depositAddr) != 1 {
			// 	return false, false, fmt.Errorf("传输费押金应包含一个地址")
			// }

			if ep.txin == nil {
				return false, false, fmt.Errorf("支付给社区但是没有TxIN，这不应该发生")
			}
			var sigScript []byte
			if ep.txin.SignatureScript != nil && len(ep.txin.SignatureScript) > 0 {
				sigScript = ep.txin.SignatureScript
			} else if ep.txin.Witness != nil && len(ep.txin.Witness) > 0 {
				sigScript = []byte(ep.txin.Witness[0])
			}
			if sigScript == nil || len(sigScript) <= 0 {
				return false, false, fmt.Errorf("给社区捐赠传输押金但是找不到证明")
			}
			params, err := txscript.ExtractFCEOperation(sigScript)
			if err != nil {
				return false, false, err
			}

			depositAddr, err := btcutil.DecodeAddress(string(buyinfo.Buyer), chain.chainParams)
			if err != nil {
				return false, false, err
			}
			proof := &btcjson.AppealTransInfo{}
			proof.FromBytes(params.Params[0].([]byte), true)
			if err := ValidateProof(params.Params[0].([]byte), &depositAddr, buyinfo.BuyHash, string(proof.Transfer), btcutil.Amount(proof.Amount)); err != nil {
				return false, false, err
			}

			// 现在，我们能证明这个签名确实是由购买者本人发起的
			if chain.bestChain.Height() < buyinfo.PayBlockHeight+100 {
				//这种情况下要证明购买者确实少发了传输费，申诉才能起作用
				//检查TxIn中的 Proof
				txinBuyinfo, err := dbFetchBuyInfoByHash(dbTx, proof.BuyHash)
				if err != nil {
					return false, false, err
				}
				if txinBuyinfo.AppealIdx >= 0 {
					return false, false, fmt.Errorf("已经有人赢得了申诉，下次请继续努力")
				}

				for _, fee := range txinBuyinfo.FinalTransFees {
					if bytes.Equal(fee.Account, proof.Transfer) && proof.Amount < fee.Fee {
						return false, false, fmt.Errorf("签名证明中的传输费小于实际发放的传输费，申诉失败")
					}
				}
				// 证明数据中的传输费大于实际发放的传输费 或者 根本找不到相应的记录，都算作申诉成功

				// 这种情况下，必须传送一部分给社区地址
				var toCommunity = false
				var totalToTransfer = int64(0)
				var proofAddr = string(proof.Transfer)
				for _, txout := range tx.MsgTx().TxOut {
					_, addresses, _, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, chain.chainParams)
					if err == nil {
						for _, addr := range addresses {
							var txoutaddr = addr.EncodeAddress()
							if txoutaddr == CommunityAddr {
								toCommunity = true
							} else if txoutaddr == proofAddr {
								totalToTransfer += txout.Value
								if totalToTransfer > buyinfo.TransFee {
									return false, false, fmt.Errorf("发送给传输者的申诉金额不能超过传输费")
								}
							} else {
								return false, false, fmt.Errorf("申诉输出地址(%s)只能是社区(%s)地址或者申诉者地址(%s)", txoutaddr, CommunityAddr, proofAddr)
							}
						}
					}
				}
				if !toCommunity {
					return false, false, fmt.Errorf("必须将剩余申诉金额发送给社区")
				}
			} else {
				if string(proof.Transfer) != string(buyinfo.Buyer) {
					return false, false, fmt.Errorf("只有购买者才能花费传输押金")
				}
			}
		}
		return false, false, nil
	},
}

type OPBlockMode uint8

const (
	OPB_CONNECT OPBlockMode = iota
	OPB_DISCONNECT
)

// 当一个块被添加到区块链的时候，检查其中的文件链信息并更新信息
var connectFuncs = []FCEExecute{
	// 在发布hash的时候更新文件hash信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if code == txscript.OP_FILE_PUBLISH {
			fileinfo.BlockHeight = block.Height()
			fileinfo.Hash = params[0].([]byte)
			fileinfo.Price = params[1].(int64)
			fileinfo.Title = params[2].([]byte)
			fileinfo.Author = params[3].([]byte)
			fileinfo.FileTag = uint8(params[4].(int64))
			return true, false, nil
		}
		return false, false, nil
	},
	// 在购买hash的时候更新购买文件hash信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if code == txscript.OP_FILE_BUY_PAY {
			txout := ep.txout
			var txidx = -1
			for idx, v := range tx.MsgTx().TxOut {
				if v == txout {
					txidx = idx
				}
			}
			if txidx == -1 {
				return false, false, fmt.Errorf("这个现象不应该出现，只是为了安全多做检查，参数错误，找不到对应的 TxOut")
			}

			_, buyaddresses, _, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, chain.chainParams)
			if err != nil {
				return false, false, err
			}

			if len(buyaddresses) != 1 {
				return false, false, fmt.Errorf("购买交易应该包含付款地址")
			}

			buyaddr := buyaddresses[0]

			buyinfo.BlockHeight = block.Height()
			buyinfo.BuyHash = params[0].([]byte)
			buyinfo.FileHash = params[1].([]byte)
			buyinfo.Buyer = []byte(buyaddr.EncodeAddress())

			//这笔押金只能在确认购买的时候花费，因此现在需要临时记录交易号和交易内索引以便消费的时候可以直接索引
			//消费以后这里存储的内容将变更为找零的交易号和交易索引
			buyinfo.TransFeeVoutIdx = uint32(txidx)
			buyinfo.TransFeeTx = tx.Hash().CloneBytes()
			buyinfo.TransFee = (txout.Value - (fileinfo.Price << 1)) >> 2
			buyinfo.AppealIdx = -1
			return false, true, nil
		}
		return false, false, nil
	},

	// 在确实支付信息以后，更新剩余押金状态
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if code == txscript.OP_FILE_BUY_TRANS {
			txout := ep.txout
			var txidx = -1
			for idx, v := range tx.MsgTx().TxOut {
				if v == txout {
					txidx = idx
				}
			}
			if txidx == -1 {
				return false, false, fmt.Errorf("这个现象不应该出现，只是为了安全多做检查，参数错误，找不到对应的 TxOut")
			}
			buyinfo.PayBlockHeight = block.Height()
			buyinfo.TransFeeVoutIdx = uint32(txidx)
			buyinfo.TransFeeTx = tx.Hash().CloneBytes()
			buyinfo.DepositValueLeft = ep.txout.Value
			return false, true, nil
		}
		return false, false, nil
	},

	// 在确认传输费的时候更新相关信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		txout := ep.txout
		if code == txscript.OP_FILE_CONFIRM_TRANSFEE {
			_, addr, _, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, chain.chainParams)
			if err != nil {
				return false, false, err
			}
			if len(addr) != 1 {
				return false, false, fmt.Errorf("确认传输费必须包含一个传输费地址: %d", len(addr))
			}
			var addrStr = addr[0].EncodeAddress()
			buyinfo.FinalTransFees = append(buyinfo.FinalTransFees, btcjson.TransFee{
				Account: []byte(addrStr),
				Fee:     txout.Value,
			})
			buyinfo.BlockHeight = block.Height() //将高度设置为当前高度以便于控制100个块之后才可以取回传输押金
			return false, true, nil
		}
		return false, false, nil
	},
}

// 当一个块从区块链移除的时候，检查文件链相关信息并更新
var disconnectFuncs = []FCEExecute{
	// 删除更新文件hash信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if code == txscript.OP_FILE_PUBLISH {
			if fileinfo != nil {
				fileinfo.Title = []byte{} //标记删除这个Fileinfo
				return true, false, nil
			}
		}
		return false, false, nil
	},
	//在支付给作者费用以后标记为已支付状态
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if pos == OPP_TxOut && code == txscript.OP_FILE_CONFIRM_BUY {
			buyinfo.PayBlockHeight = 0
			return false, true, nil
		}
		return false, false, nil
	},
	// 删除购买信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		if code == txscript.OP_FILE_BUY_TRANS {
			if buyinfo != nil {
				buyinfo.FileHash = []byte{} //标记删除这个Fileinfo
				return false, true, nil
			}
		}
		return false, false, nil
	},
	// 删除传输费相关信息
	func(code uint8, params []interface{}, pos OPCodePOS, fileinfo *btcjson.FileHashInfo, buyinfo *btcjson.BuyHashInfo, dbTx database.Tx, block *btcutil.Block, tx *btcutil.Tx, ep ExecutePoint) (bool, bool, error) {
		txout := ep.txout
		if code == txscript.OP_FILE_CONFIRM_TRANSFEE {
			_, addr, _, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, chain.chainParams)
			if err != nil {
				return false, false, err
			}
			if len(addr) != 1 {
				return false, false, fmt.Errorf("确认传输费必须包含一个传输费地址: %d", len(addr))
			}
			var addrStr = addr[0].EncodeAddress()
			for idx, value := range buyinfo.FinalTransFees {
				if bytes.Equal([]byte(addrStr), value.Account) {
					buyinfo.FinalTransFees = append(buyinfo.FinalTransFees[:idx], buyinfo.FinalTransFees[idx+1:]...)
					break
				}
			}
			return false, true, nil
		}
		return false, false, nil
	},
}

func initFileInfoFromTransactionTxOuts(eps []ExecutePoint) (*btcjson.FileHashInfo, *btcjson.BuyHashInfo, error) {
	var fileinfo *btcjson.FileHashInfo
	var buyinfo *btcjson.BuyHashInfo
	for _, ep := range eps {
		out := ep.txout
		params, err := txscript.ExtractFCEOperation(out.PkScript)
		if params != nil {
			log.Tracef("初始化文件链参数:(%d)vout:(%f): 文件链参数: %s", ep.pos, btcutil.Amount(ep.txout.Value).ToBTC(), params.String())
			if err := chain.db.View(func(dbTx database.Tx) error {
				if codedefine, ok := txscript.FCEOpCodeDefine[params.Code]; ok {
					if codedefine.BuyHashValid {
						buyhash := params.Params[0].([]byte) //如果buyhash有效，一定是第一个参数
						if buyinfo == nil && params.Code != txscript.OP_FILE_BUY_PAY {
							if buyinfo, err = dbFetchBuyInfoByHash(dbTx, buyhash); err != nil {
								return err
							}
							if fileinfo, err = dbFetchFileInfoByHash(dbTx, buyinfo.FileHash); err != nil {
								return err
							}
						} else if buyinfo == nil && params.Code == txscript.OP_FILE_BUY_PAY {
							// 购买文件的时候，购买参数还没有初始化，文件hash又是第二个参数，所以做一点特殊处理
							if fileinfo, err = dbFetchFileInfoByHash(dbTx, params.Params[1].([]byte)); err != nil {
								return err
							}
						} else {
							if buyinfo != nil && !bytes.Equal(buyinfo.BuyHash, buyhash) {
								return fmt.Errorf("在一个事务中不允许出现两次购买hash: %s vs %s: %s", hex.EncodeToString(buyinfo.BuyHash), hex.EncodeToString(buyhash), params.String())
							}
						}
					} else {
						//如果文件hash有效，则第一个参数为文件hash
						filehash := params.Params[0].([]byte)
						if fileinfo == nil && params.Code != txscript.OP_FILE_PUBLISH { // 除非是创建新hash，否则就需要初始化哈希
							if fileinfo, err = dbFetchFileInfoByHash(dbTx, filehash); err != nil {
								return err
							}
						} else {
							if fileinfo != nil && !bytes.Equal(fileinfo.Hash, filehash) {
								return fmt.Errorf("在一个事务中不允许出现两种文件hash")
							}
						}
					}
				}
				return nil
			}); err != nil {
				return nil, nil, err
			}
		}
	}
	return fileinfo, buyinfo, nil
}

// 收到一个 Tx 以后，每个节点都会判断是否应该接受这个 Tx，在比特币原有的判断逻辑之后，新加文件链的判断逻辑
func FileChainMaybeAcceptTransaction(utxos *UtxoViewpoint, tx *btcutil.Tx) error {
	//一条事务会有多个TxOut，但是这些TxOut不应该涉及多个文件哈希或者多次购买
	var fileinfo *btcjson.FileHashInfo
	var buyinfo *btcjson.BuyHashInfo
	var err error

	allExecutePoint := make([]ExecutePoint, 0)
	for _, out := range tx.MsgTx().TxOut {
		allExecutePoint = append(allExecutePoint, ExecutePoint{
			pos:   OPP_TxOut,
			txout: out,
		})
	}
	for _, txin := range tx.MsgTx().TxIn {
		entry := utxos.LookupEntry(txin.PreviousOutPoint)
		if entry != nil {
			allExecutePoint = append(allExecutePoint, ExecutePoint{
				txin: txin,
				pos:  OPP_TxPrevOut,
				txout: &wire.TxOut{
					Value:    entry.amount,
					PkScript: entry.pkScript,
				},
			})
		}
	}

	fileinfo, buyinfo, err = initFileInfoFromTransactionTxOuts(allExecutePoint)
	if err != nil {
		return err
	}

	for _, out := range allExecutePoint {
		params, _ := txscript.ExtractFCEOperation(out.txout.PkScript)
		if params != nil {
			log.Tracef("接受交易之前检查文件链参数:(%d)vout:(%f): 文件链参数: %s", out.pos, btcutil.Amount(out.txout.Value).ToBTC(), params.String())
			if err := chain.db.View(func(dbTx database.Tx) error {
				for _, reject := range FCERejectRules {
					if _, _, err := reject(params.Code, params.Params, out.pos, fileinfo, buyinfo, dbTx, nil, tx, out); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// 收到购买方的传输费证明以后，确认签名是否有效
func ValidateProof(proof []byte, buyer *btcutil.Address, buyhash []byte, transfer string, amount btcutil.Amount) error {
	var info btcjson.AppealTransInfo
	if err := info.FromBytes(proof, true); err != nil {
		return err
	}
	if info.Amount < int64(amount) {
		return fmt.Errorf("证明中的金额 %d 少于需要证明的金额: %d", info.Amount, int64(amount))
	}

	if !bytes.Equal(info.BuyHash, buyhash) {
		return fmt.Errorf("证明中的文件hash不等于需要被证明的哈希")
	}

	if string(info.Transfer) != transfer {
		return fmt.Errorf("证明中的传输者不等于需要被证明的传输者")
	}

	sigdata := info.ToBytes(false)
	messageHash := chainhash.DoubleHashB(sigdata)
	//从证明的签名数据中解析出地址
	pubkey, _, err := btcec.RecoverCompact(btcec.S256(), info.Sig, messageHash)
	if err != nil {
		return err
	}
	pubKeyHash := btcutil.Hash160(pubkey.SerializeCompressed())
	log.Info("签名中包含的公钥是:", hex.EncodeToString(pubKeyHash))
	switch (*buyer).(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		if !bytes.Equal((*buyer).ScriptAddress(), pubKeyHash) {
			return fmt.Errorf("证明签名校验出错，证明数据里包含的签名不是接收者的地址不吻合 %s != %s", hex.EncodeToString((*buyer).ScriptAddress()), hex.EncodeToString(pubKeyHash))
		}
	default:
		return fmt.Errorf("不支持的地址格式")
	}
	return nil
}

func dbFetchBuyInfoByHash(dbTx database.Tx, hash []byte) (*btcjson.BuyHashInfo, error) {
	meta := dbTx.Metadata()
	bucket := meta.Bucket(fileChainBuyHashBucketName)
	bs := bucket.Get(hash)
	if bs == nil {
		return nil, fmt.Errorf("找不到哈希对应的购买记录:%s", hex.EncodeToString(hash))
	}
	info := &btcjson.BuyHashInfo{}
	info.FromBytes(bs)
	return info, nil
}

func dbFetchFileInfoByHash(dbTx database.Tx, hash []byte) (*btcjson.FileHashInfo, error) {
	meta := dbTx.Metadata()
	bucket := meta.Bucket(fileChainHashBucketName)
	bs := bucket.Get(hash)
	if bs == nil {
		return nil, fmt.Errorf("找不到哈希对应的文件记录:%s", hex.EncodeToString(hash))
	}
	info := &btcjson.FileHashInfo{}
	info.FromBytes(bs)
	return info, nil
}

func dbPutBuyHashInfo(dbTx database.Tx, info *btcjson.BuyHashInfo) error {
	bs := info.ToBytes()
	// Store the current best chain state into the database.
	meta := dbTx.Metadata()
	bucket := meta.Bucket(fileChainBuyHashBucketName)
	return bucket.Put(info.BuyHash, bs)
}

func dbPutFileHashInfo(dbTx database.Tx, info *btcjson.FileHashInfo) error {
	bs := info.ToBytes()
	// Store the current best chain state into the database.
	meta := dbTx.Metadata()
	bucket := meta.Bucket(fileChainHashBucketName)
	return bucket.Put(info.Hash, bs)
}

func FileChainOnChainBlockChange(block *btcutil.Block, mode OPBlockMode) error {
	for _, tx := range block.Transactions() {
		eps := make([]ExecutePoint, 0)
		for _, out := range tx.MsgTx().TxOut {
			eps = append(eps, ExecutePoint{
				pos:   OPP_TxOut,
				txout: out,
			})
		}
		fileinfo, buyinfo, err := initFileInfoFromTransactionTxOuts(eps)
		fileinfoNeedUpdate := false
		buyinfoNeedUpdate := false
		if err != nil {
			return err
		}
		if fileinfo == nil {
			fileinfo = &btcjson.FileHashInfo{}
		}
		if buyinfo == nil {
			buyinfo = &btcjson.BuyHashInfo{}
		}
		totalTransfee := buyinfo.TotalFee()
		for _, out := range tx.MsgTx().TxOut {
			params, _ := txscript.ExtractFCEOperation(out.PkScript)
			if params != nil {
				if mode == OPB_CONNECT {
					log.Tracef("连接新的文件块的时候检查文件链参数:vout:(%f): 文件链参数:%s", btcutil.Amount(out.Value).ToBTC(), params.String())
				} else {
					log.Tracef("移除无效文件块时候检查文件链参数:vout:(%f): 文件链参数:%s", btcutil.Amount(out.Value).ToBTC(), params.String())
				}
				if err := chain.db.View(func(dbTx database.Tx) error {
					funcs := connectFuncs
					if mode == OPB_DISCONNECT {
						funcs = disconnectFuncs
					}
					for _, connectFunc := range funcs {
						dirtyfile, dirtybuy, err := connectFunc(params.Code, params.Params, OPP_TxOut, fileinfo, buyinfo, dbTx, block, tx, ExecutePoint{
							pos:   OPP_TxOut,
							txout: out,
						})
						if err != nil {
							return err
						}
						if dirtyfile {
							fileinfoNeedUpdate = true
						}
						if dirtybuy {
							buyinfoNeedUpdate = true
						}
					}
					return nil
				}); err != nil {
					return err
				}
			}
		}
		if buyinfoNeedUpdate || fileinfoNeedUpdate {
			if err := chain.db.Update(func(dbTx database.Tx) error {
				if buyinfoNeedUpdate {
					if len(buyinfo.FileHash) == 0 { //标记此信息已无效的标记
						meta := dbTx.Metadata()
						bucket := meta.Bucket(fileChainBuyHashBucketName)
						if err := bucket.Delete(buyinfo.BuyHash); err != nil {
							return err
						}
					} else if err := dbPutBuyHashInfo(dbTx, buyinfo); err != nil {
						return err
					}
				}
				if buyinfo != nil && totalTransfee != buyinfo.TotalFee() {
					totalTransfee = buyinfo.TotalFee()
					if totalTransfee >= buyinfo.TransFee>>1 {
						fileinfo.DownloadSucc++
					} else {
						fileinfo.DownloadFail++
					}
					fileinfoNeedUpdate = true
				}
				if fileinfoNeedUpdate {
					if len(fileinfo.Title) == 0 {
						meta := dbTx.Metadata()
						bucket := meta.Bucket(fileChainHashBucketName)
						if err := bucket.Delete(fileinfo.Hash); err != nil {
							return err
						}
					} else if err := dbPutFileHashInfo(dbTx, fileinfo); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

//初始化文件存储数据库
func FileChainInitStore(b *BlockChain, needCreate bool) error {
	chain = b
	if needCreate {
		err := b.db.Update(func(dbTx database.Tx) error {
			meta := dbTx.Metadata()

			// Create the bucket that houses the block index data.
			_, err := meta.CreateBucket(fileChainHashBucketName)
			if err != nil {
				return err
			}
			// Create the bucket that houses the block index data.
			_, err = meta.CreateBucket(fileChainBuyHashBucketName)
			if err != nil {
				return err
			}
			return nil
		})
		return err
	}
	return nil
}
