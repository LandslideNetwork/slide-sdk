package psql

import (
	"github.com/consideritdone/landslidevm/utils/state/indexer"
	"github.com/consideritdone/landslidevm/utils/state/txindex"
)

var (
	_ indexer.BlockIndexer = BackportBlockIndexer{}
	_ txindex.TxIndexer    = BackportTxIndexer{}
)
