package chain

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/types"
)

var DefaultChainIndexCacheSize = 32 << 15

func init() {
	if s := os.Getenv("CHAIN_INDEX_CACHE"); s != "" {
		lcic, err := strconv.Atoi(s)
		if err != nil {
			log.Errorf("failed to parse 'CHAIN_INDEX_CACHE' env var: %s", err)
		}
		DefaultChainIndexCacheSize = lcic
	}
}

// ChainIndex tipset height index, used to getting tipset by height quickly
type ChainIndex struct { //nolint
	indexCacheLk sync.Mutex
	indexCache   map[types.TipSetKey]*lbEntry

	loadTipSet loadTipSetFunc

	skipLength abi.ChainEpoch
}

// NewChainIndex return a new chain index with arc cache
func NewChainIndex(lts loadTipSetFunc) *ChainIndex {
	return &ChainIndex{
		indexCache: make(map[types.TipSetKey]*lbEntry, DefaultChainIndexCacheSize),
		loadTipSet: lts,
		skipLength: 20,
	}
}

type lbEntry struct {
	targetHeight abi.ChainEpoch
	target       types.TipSetKey
}

// GetTipSetByHeight get tipset at specify height from specify tipset
// the tipset within the skiplength is directly obtained by reading the database.
// if the height difference exceeds the skiplength, the tipset is read from caching.
// if the caching fails, the tipset is obtained by reading the database and updating the cache
func (ci *ChainIndex) GetTipSetByHeight(ctx context.Context, from *types.TipSet, to abi.ChainEpoch) (*types.TipSet, error) {
	if from.Height()-to <= ci.skipLength {
		return ci.walkBack(ctx, from, to)
	}

	rounded, err := ci.roundDown(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to round down: %w", err)
	}

	ci.indexCacheLk.Lock()
	defer ci.indexCacheLk.Unlock()
	cur := rounded.Key()
	for {
		lbe, ok := ci.indexCache[cur]
		if !ok {
			fc, err := ci.fillCache(ctx, cur)
			if err != nil {
				return nil, fmt.Errorf("failed to fill cache: %w", err)
			}
			lbe = fc
		}

		if to == lbe.targetHeight {
			ts, err := ci.loadTipSet(ctx, lbe.target)
			if err != nil {
				return nil, fmt.Errorf("failed to load tipset: %w", err)
			}

			return ts, nil
		}
		if to > lbe.targetHeight {
			ts, err := ci.loadTipSet(ctx, cur)
			if err != nil {
				return nil, fmt.Errorf("failed to load tipset: %w", err)
			}
			return ci.walkBack(ctx, ts, to)
		}

		cur = lbe.target
	}
}

// GetTipsetByHeightWithoutCache get the tipset of specific height by reading the database directly
func (ci *ChainIndex) GetTipsetByHeightWithoutCache(ctx context.Context, from *types.TipSet, to abi.ChainEpoch) (*types.TipSet, error) {
	return ci.walkBack(ctx, from, to)
}

// Caller must hold indexCacheLk
func (ci *ChainIndex) fillCache(ctx context.Context, tsk types.TipSetKey) (*lbEntry, error) {
	ts, err := ci.loadTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("failed to load tipset: %w", err)
	}

	if ts.Height() == 0 {
		return &lbEntry{
			targetHeight: 0,
			target:       tsk,
		}, nil
	}

	// will either be equal to ts.Height, or at least > ts.Parent.Height()
	rheight := ci.roundHeight(ts.Height())

	parent, err := ci.loadTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, err
	}

	rheight -= ci.skipLength
	if rheight < 0 {
		rheight = 0
	}

	var skipTarget *types.TipSet
	if parent.Height() < rheight {
		skipTarget = parent
	} else {
		skipTarget, err = ci.walkBack(ctx, parent, rheight)
		if err != nil {
			return nil, fmt.Errorf("fillCache walkback: %w", err)
		}
	}

	lbe := &lbEntry{
		targetHeight: skipTarget.Height(),
		target:       skipTarget.Key(),
	}
	ci.indexCache[tsk] = lbe

	return lbe, nil
}

// floors to nearest skipLength multiple
func (ci *ChainIndex) roundHeight(h abi.ChainEpoch) abi.ChainEpoch {
	return (h / ci.skipLength) * ci.skipLength
}

func (ci *ChainIndex) roundDown(ctx context.Context, ts *types.TipSet) (*types.TipSet, error) {
	target := ci.roundHeight(ts.Height())

	rounded, err := ci.walkBack(ctx, ts, target)
	if err != nil {
		return nil, fmt.Errorf("failed to walk back: %w", err)
	}

	return rounded, nil
}

func (ci *ChainIndex) walkBack(ctx context.Context, from *types.TipSet, to abi.ChainEpoch) (*types.TipSet, error) {
	if to > from.Height() {
		return nil, fmt.Errorf("looking for tipset with height greater than start point")
	}

	if to == from.Height() {
		return from, nil
	}

	ts := from

	for {
		pts, err := ci.loadTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to load tipset: %w", err)
		}

		if to > pts.Height() {
			// in case pts is lower than the epoch we're looking for (null blocks)
			// return a tipset above that height
			return ts, nil
		}
		if to == pts.Height() {
			return pts, nil
		}

		ts = pts
	}
}
