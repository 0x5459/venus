// FETCHED FROM LOTUS: builtin/multisig/message.go.template

package multisig

import (
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	builtin8 "github.com/filecoin-project/specs-actors/v8/actors/builtin"
	init8 "github.com/filecoin-project/specs-actors/v8/actors/builtin/init"
	multisig8 "github.com/filecoin-project/specs-actors/v8/actors/builtin/multisig"

	"github.com/filecoin-project/venus/venus-shared/actors"
	init_ "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	types "github.com/filecoin-project/venus/venus-shared/internal"
)

type message8 struct{ message0 }

func (m message8) Create(
	signers []address.Address, threshold uint64,
	unlockStart, unlockDuration abi.ChainEpoch,
	initialAmount abi.TokenAmount,
) (*types.Message, error) {

	lenAddrs := uint64(len(signers))

	if lenAddrs < threshold {
		return nil, xerrors.Errorf("cannot require signing of more addresses than provided for multisig")
	}

	if threshold == 0 {
		threshold = lenAddrs
	}

	if m.from == address.Undef {
		return nil, xerrors.Errorf("must provide source address")
	}

	// Set up constructor parameters for multisig
	msigParams := &multisig8.ConstructorParams{
		Signers:               signers,
		NumApprovalsThreshold: threshold,
		UnlockDuration:        unlockDuration,
		StartEpoch:            unlockStart,
	}

	enc, actErr := actors.SerializeParams(msigParams)
	if actErr != nil {
		return nil, actErr
	}

	actorCodeID, ok := actors.GetActorCodeID(actors.Version8, "multisig")
	if !ok {
		return nil, xerrors.Errorf("error getting actor multisig code id for actor version %d", 8)
	}

	// new actors are created by invoking 'exec' on the init actor with the constructor params
	execParams := &init8.ExecParams{
		CodeCID:           actorCodeID,
		ConstructorParams: enc,
	}

	enc, actErr = actors.SerializeParams(execParams)
	if actErr != nil {
		return nil, actErr
	}

	return &types.Message{
		To:     init_.Address,
		From:   m.from,
		Method: builtin8.MethodsInit.Exec,
		Params: enc,
		Value:  initialAmount,
	}, nil
}
