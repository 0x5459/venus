package eth

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(ctx context.Context,
	cfg *config.Config,
	chainModule *chain.ChainSubmodule,
	mpoolModule *mpool.MessagePoolSubmodule,
	sqlitePath string,
) (*EthSubModule, error) {
	ctx, cancel := context.WithCancel(ctx)
	em := &EthSubModule{
		cfg:         cfg,
		chainModule: chainModule,
		mpoolModule: mpoolModule,
		sqlitePath:  sqlitePath,
		ctx:         ctx,
		cancel:      cancel,
	}
	ee, err := newEthEventAPI(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("create eth event api error %v", err)
	}
	em.ethEventAPI = ee

	em.ethAPIAdapter = &ethAPIDummy{}
	if em.cfg.FevmConfig.EnableEthRPC || constants.FevmEnableEthRPC {
		log.Debug("enable eth rpc")
		em.ethAPIAdapter, err = newEthAPI(em)
		if err != nil {
			return nil, err
		}
	}

	return em, nil
}

type EthSubModule struct { // nolint
	cfg         *config.Config
	chainModule *chain.ChainSubmodule
	mpoolModule *mpool.MessagePoolSubmodule
	sqlitePath  string

	ethEventAPI   *ethEventAPI
	ethAPIAdapter ethAPIAdapter

	ctx    context.Context
	cancel context.CancelFunc
}

func (em *EthSubModule) Start(_ context.Context) error {
	if err := em.ethEventAPI.Start(em.ctx); err != nil {
		return err
	}

	return em.ethAPIAdapter.start(em.ctx)
}

func (em *EthSubModule) Close(ctx context.Context) error {
	// exit waitForMpoolUpdates, avoid panic
	em.cancel()

	if err := em.ethEventAPI.Close(ctx); err != nil {
		return err
	}

	return em.ethAPIAdapter.close()
}

type ethAPIAdapter interface {
	v1api.IETH
	start(ctx context.Context) error
	close() error
}

type fullETHAPI struct {
	v1api.IETH
	*ethEventAPI
}

var _ v1api.IETH = (*fullETHAPI)(nil)

func (em *EthSubModule) API() v1api.FullETH {
	return &fullETHAPI{
		IETH:        em.ethAPIAdapter,
		ethEventAPI: em.ethEventAPI,
	}
}
