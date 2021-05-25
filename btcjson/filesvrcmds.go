package btcjson

import (
	"bytes"
	"encoding/binary"

	"github.com/filechain/filechain/btcutil"
)

// 发布一个文件哈希，
// 必选参数文件哈希是 将文件内容抽象成一个哈希组，然后再加上文件元数据组成一个不超过256K的文件，称为种子文件
// 将这个文件先sha256(2)，再sha256(1)，得到一个20个字节的哈希值
// 输入的价格是浮点类型1.5代表 150000000聪
// 标题要尽量端，避免超出比特币对单个script的长度限制
// 作者地址是可选参数，如果有人购买，收入将进入这个地址，默认为当前钱包的默认地址
type PublishHashCmd struct {
	Hash          string
	Price         float64
	Title         string
	IsSexually    *bool
	IsPolitically *bool
	IsViolence    *bool
	IsOriginal    *bool
	AuthorAddr    *string
}

// 传输证明中需要包含数据和购买者签名
type GetProofCmd struct {
	BuyHash  string  //要给哪次购买做签名
	Transfer string  //要给哪个传输地址做签名
	Amount   float64 //要证明这个传输者传输了多少
	Account  *string //使用哪个账户签名，默认为default
}

// 通过RPC查询主链获取已经发布的hash信息
type ListHashCmd struct {
	Limit  *int64
	Offset *int64
}

type ListSingleHashCmd struct {
	Hash string
}

type AppealTransInfo struct {
	BuyHash  []byte
	Transfer []byte
	Amount   int64
	Sig      []byte
}

func (info *AppealTransInfo) ToBytes(includeSig bool) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.BuyHash)))
	binary.Write(&buf, binary.LittleEndian, info.BuyHash)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.Transfer)))
	binary.Write(&buf, binary.LittleEndian, info.Transfer)
	binary.Write(&buf, binary.LittleEndian, info.Amount)
	if includeSig {
		binary.Write(&buf, binary.LittleEndian, uint16(len(info.Sig)))
		binary.Write(&buf, binary.LittleEndian, info.Sig)
	}
	return buf.Bytes()
}

func (info *AppealTransInfo) FromBytes(bs []byte, includeSig bool) error {
	var size uint16
	buf := bytes.NewBuffer(bs)
	binary.Read(buf, binary.LittleEndian, &size)
	info.BuyHash = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.BuyHash)
	binary.Read(buf, binary.LittleEndian, &size)
	info.Transfer = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.Transfer)
	binary.Read(buf, binary.LittleEndian, &info.Amount)
	if includeSig {
		binary.Read(buf, binary.LittleEndian, &size)
		info.Sig = make([]byte, size)
		binary.Read(buf, binary.LittleEndian, info.Sig)
	}
	return nil
}

type GetBuyHashCmd struct {
	Hash string
}

type BuyHashInfo struct {
	BlockHeight      int32
	PayBlockHeight   int32
	BuyHash          []byte
	FileHash         []byte
	Buyer            []byte
	TransFee         int64
	TransFeeTx       []byte
	TransFeeVoutIdx  uint32
	DepositValueLeft int64
	AppealIdx        int32 //如果有多人申诉，只会有一个获胜者，获胜者必然在 FinalTransFee 里，这里记录获胜者的索引
	FinalTransFees   []TransFee
}

func (info *BuyHashInfo) TotalFee() int64 {
	var result int64
	for _, v := range info.FinalTransFees {
		result += v.Fee
	}
	return result
}

type BuyHashInfoRsp struct {
	BlockHeight      int32
	BuyHash          string
	Buyer            string
	FileHash         string
	TransFee         float64
	TransFeeTx       string
	TransFeeVoutIdx  uint32
	DepositValueLeft float64
	FinalTransFees   map[string]float64
}

func (info *BuyHashInfo) ToBytes() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, info.BlockHeight)
	binary.Write(&buf, binary.LittleEndian, info.PayBlockHeight)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.BuyHash)))
	binary.Write(&buf, binary.LittleEndian, info.BuyHash)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.FileHash)))
	binary.Write(&buf, binary.LittleEndian, info.FileHash)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.Buyer)))
	binary.Write(&buf, binary.LittleEndian, info.Buyer)
	binary.Write(&buf, binary.LittleEndian, info.TransFee)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.TransFeeTx)))
	binary.Write(&buf, binary.LittleEndian, info.TransFeeTx)
	binary.Write(&buf, binary.LittleEndian, info.TransFeeVoutIdx)
	binary.Write(&buf, binary.LittleEndian, info.DepositValueLeft)
	binary.Write(&buf, binary.LittleEndian, info.AppealIdx)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.FinalTransFees)))
	for _, fee := range info.FinalTransFees {
		binary.Write(&buf, binary.LittleEndian, uint16(len(fee.Account)))
		binary.Write(&buf, binary.LittleEndian, fee.Account)
		binary.Write(&buf, binary.LittleEndian, fee.Fee)
	}
	return buf.Bytes()
}

func (info *BuyHashInfo) FromBytes(bs []byte) error {
	var size uint16
	buf := bytes.NewBuffer(bs)
	binary.Read(buf, binary.LittleEndian, &info.BlockHeight)
	binary.Read(buf, binary.LittleEndian, &info.PayBlockHeight)
	binary.Read(buf, binary.LittleEndian, &size)
	info.BuyHash = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.BuyHash)
	binary.Read(buf, binary.LittleEndian, &size)
	info.FileHash = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.FileHash)
	binary.Read(buf, binary.LittleEndian, &size)
	info.Buyer = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.Buyer)
	binary.Read(buf, binary.LittleEndian, &info.TransFee)
	binary.Read(buf, binary.LittleEndian, &size)
	info.TransFeeTx = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.TransFeeTx)
	binary.Read(buf, binary.LittleEndian, &info.TransFeeVoutIdx)
	binary.Read(buf, binary.LittleEndian, &info.DepositValueLeft)
	binary.Read(buf, binary.LittleEndian, &info.AppealIdx)
	binary.Read(buf, binary.LittleEndian, &size)
	info.FinalTransFees = make([]TransFee, 0, size)
	for i := uint16(0); i < size; i++ {
		var subsize uint16
		var fee TransFee
		binary.Read(buf, binary.LittleEndian, &subsize)
		fee.Account = make([]byte, subsize)
		binary.Read(buf, binary.LittleEndian, fee.Account)
		binary.Read(buf, binary.LittleEndian, &fee.Fee)
		info.FinalTransFees = append(info.FinalTransFees, fee)
	}
	return nil
}

const (
	Sexually = iota
	Politically
	Violence
	Original
)

type FileHashInfo struct {
	BlockHeight  int32
	Hash         []byte
	Title        []byte
	Price        int64
	Author       []byte
	FileTag      uint8
	DownloadSucc uint32
	DownloadFail uint32
	LastSuccTime int64
}

type FileHashInfoRsp struct {
	BlockHeight   int32
	Hash          string
	Title         string
	Price         float64
	Author        string
	DownloadSucc  uint32
	DownloadFail  uint32
	LastSuccTime  int64
	IsSexually    bool
	IsPolitically bool
	IsViolence    bool
	IsOriginal    bool
}

func (info *FileHashInfo) IsSexually() bool {
	return info.FileTag&(1<<Sexually) == 1<<Sexually
}

func (info *FileHashInfo) IsPolitically() bool {
	return info.FileTag&(1<<Politically) == 1<<Politically
}

func (info *FileHashInfo) IsViolence() bool {
	return info.FileTag&(1<<Violence) == 1<<Violence
}

func (info *FileHashInfo) IsOriginal() bool {
	return info.FileTag&(1<<Original) == 1<<Original
}

func (info *FileHashInfo) ToBytes() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, info.BlockHeight)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.Hash)))
	binary.Write(&buf, binary.LittleEndian, info.Hash)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.Title)))
	binary.Write(&buf, binary.LittleEndian, info.Title)
	binary.Write(&buf, binary.LittleEndian, info.Price)
	binary.Write(&buf, binary.LittleEndian, uint16(len(info.Author)))
	binary.Write(&buf, binary.LittleEndian, info.Author)
	binary.Write(&buf, binary.LittleEndian, info.FileTag)
	binary.Write(&buf, binary.LittleEndian, info.DownloadSucc)
	binary.Write(&buf, binary.LittleEndian, info.DownloadFail)
	binary.Write(&buf, binary.LittleEndian, info.LastSuccTime)
	return buf.Bytes()
}

func (info *FileHashInfo) FromBytes(bs []byte) error {
	var size uint16
	buf := bytes.NewBuffer(bs)
	binary.Read(buf, binary.LittleEndian, &info.BlockHeight)
	binary.Read(buf, binary.LittleEndian, &size)
	info.Hash = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.Hash)
	binary.Read(buf, binary.LittleEndian, &size)
	info.Title = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.Title)
	binary.Read(buf, binary.LittleEndian, &info.Price)
	binary.Read(buf, binary.LittleEndian, &size)
	info.Author = make([]byte, size)
	binary.Read(buf, binary.LittleEndian, info.Author)
	binary.Read(buf, binary.LittleEndian, &info.FileTag)
	binary.Read(buf, binary.LittleEndian, &info.DownloadSucc)
	binary.Read(buf, binary.LittleEndian, &info.DownloadFail)
	binary.Read(buf, binary.LittleEndian, &info.LastSuccTime)
	return nil
}

type ListHashResult struct {
	Total  int64
	Result []*FileHashInfoRsp
}

// 是上述接口的内部形式
type PublicHash struct {
	Hash        []byte
	BlockCount  uint32
	Title       []byte
	Price       btcutil.Amount
	TxOut       btcutil.Amount
	Author      []byte
	FileTag     uint8
	DepositAddr btcutil.Address
}

// 购买一个文件哈希
// 指定要购买的文件hash，指定愿意出的传输费，这个时候，在链下，购买者已经得到了种子文件
// 种子文件指定了文件一共有多少个字节的内容
// 指定传输费以后，可以算出来平均单个字节的传输费，传输者可能会按照优先传输高价的顺序排序
type BuyHashCmd struct {
	Hash     string  // 文件hash，要购买哪个文件
	TransFee float64 // 传输费必须大于0
}

type TransFee struct {
	Account []byte `json:"account"`
	Fee     int64  `json:"fee"`
}

type TransFeeReq struct {
	Account string  `json:"account"`
	Fee     float64 `json:"fee"`
}

type ConfirmBuyCmd struct {
	Hash string        // 购买hash，定位一次购买行为
	Fees []TransFeeReq // 传输费，从某个人那里得到了多少个块。传输费实时到账
}

// 传输费申诉：传输者在传输过程中会得到购买者签名的传输费，
// 签名内容为 购买hash + 传输者地址 + 传输费,如果在 ConfirmBuyCmd 中确认的传输费不对，传输者可以通过该签名内容进行申诉
// 如果申诉得到确认，申诉者将得到单倍传输押金，社区将得到3倍传输押金
// 矿工会验证该签名确实是传输押金获得者的地址所签发，并且内容确实是当前的购买hash和购买地址，且传输给某地址的传输费确实小于签名中的传输费
// 整个判定信息会比较长，应该通过隔离见证的方式节省存储空间
// Proof的内容为 公钥、签名、购买hash、传输者地址、传输费
type AppealTransferFeeCmd struct {
	Proof string
}

func init() {
	// The commands in this file are only usable with a wallet server.
	flags := UFWalletOnly

	MustRegisterCmd("listfilehash", (*ListHashCmd)(nil), flags)
	MustRegisterCmd("getfilehashinfo", (*ListSingleHashCmd)(nil), flags)
	MustRegisterCmd("getbuyhashinfo", (*GetBuyHashCmd)(nil), flags)
	MustRegisterCmd("publishhash", (*PublishHashCmd)(nil), flags)
	MustRegisterCmd("getproof", (*GetProofCmd)(nil), flags)
	MustRegisterCmd("buyhash", (*BuyHashCmd)(nil), flags)
	MustRegisterCmd("confirmbuy", (*ConfirmBuyCmd)(nil), flags)
	MustRegisterCmd("appealtransfee", (*AppealTransferFeeCmd)(nil), flags)
	MustRegisterCmd("createwallet", (*CreateWalletCmd)(nil), flags)
}
