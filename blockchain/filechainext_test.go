package blockchain_test

// 钱包客户端将交易发给矿工是，这是第一道屏障（目前看来似乎也是唯一的一道屏障，总让我感觉不太安全）
// 文件链不像纯比特币交易一样不需要依赖世界状态，这方面更像是以太坊，但是又不想实现太复杂的世界状态（存储空间和复杂度考量）
// 在一个事务提交到某个全节点的时候，这个全节点维护了一套简单的文件链状态体系，这个文件链状态唯一的依赖就是链上数据，而且只依赖
// 链上数据，如果两个节点拥有同样的链上数据，可以认为他们的状态是一致，因为区块链最终链上数据会趋于一致，因此他们的状态也将趋于一致
//
func FileChainExt_FileChainMaybeAcceptTransaction() {
}
