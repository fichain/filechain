package btcjson_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/fichain/filechain/btcjson"
)

func TestAppealTransInfo(t *testing.T) {
	var info = &btcjson.AppealTransInfo{}
	testkey1, _ := hex.DecodeString("12345678")
	testkey3, _ := hex.DecodeString("34567890")
	testkey4, _ := hex.DecodeString("45678901")
	info.Amount = 10000
	info.BuyHash = testkey1
	info.Sig = testkey3
	info.Transfer = testkey4
	bs := info.ToBytes(true)
	info2 := &btcjson.AppealTransInfo{}
	info2.FromBytes(bs, true)
	if !bytes.Equal(info.BuyHash, info2.BuyHash) {
		t.Errorf("BuyHash不相等")
	}
	if !bytes.Equal(info.Sig, info2.Sig) {
		t.Errorf("Sig不相等")
	}
}
