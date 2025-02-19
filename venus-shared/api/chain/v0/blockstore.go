package v0

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	blocks "github.com/ipfs/go-libipfs/blocks"
)

type IBlockStore interface {
	ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error)                      //perm:read
	ChainDeleteObj(ctx context.Context, obj cid.Cid) error                              //perm:admin
	ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error)                         //perm:read
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (types.ObjStat, error) //perm:read
	// ChainPutObj puts a given object into the block store
	ChainPutObj(context.Context, blocks.Block) error //perm:admin
}
