package stake

import (
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	"github.com/cosmos/cosmos-sdk/x/mock"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/sidechain"
)

// getMockApp returns an initialized mock application for this module.
func getMockApp(t *testing.T) (*mock.App, Keeper) {
	mApp := mock.NewApp()

	RegisterCodec(mApp.Cdc)

	keyStake := sdk.NewKVStoreKey("stake")
	keyStakeReward := sdk.NewKVStoreKey("stake_reward")
	tkeyStake := sdk.NewTransientStoreKey("transient_stake")
	keyParams := sdk.NewKVStoreKey("params")
	tkeyParams := sdk.NewTransientStoreKey("transient_params")
	keyIbc := sdk.NewKVStoreKey("ibc")
	keySideChain := sdk.NewKVStoreKey("sc")

	bankKeeper := bank.NewBaseKeeper(mApp.AccountKeeper)
	pk := params.NewKeeper(mApp.Cdc, keyParams, tkeyParams)
	scKeeper := sidechain.NewKeeper(keySideChain, pk.Subspace(sidechain.DefaultParamspace), mApp.Cdc)
	ibcKeeper := ibc.NewKeeper(keyIbc, pk.Subspace(ibc.DefaultParamspace), ibc.DefaultCodespace, scKeeper)
	keeper := NewKeeper(mApp.Cdc, keyStake, keyStakeReward, tkeyStake, bankKeeper, nil, pk.Subspace(DefaultParamspace), mApp.RegisterCodespace(DefaultCodespace))
	keeper.SetupForSideChain(&scKeeper, &ibcKeeper)

	mApp.Router().AddRoute("stake", NewStakeHandler(keeper))
	mApp.SetEndBlocker(getEndBlocker(keeper))
	mApp.SetInitChainer(getInitChainer(mApp, keeper))

	require.NoError(t, mApp.CompleteSetup(keyStake, tkeyStake, keyParams, tkeyParams))
	return mApp, keeper
}

// getEndBlocker returns a stake endblocker.
func getEndBlocker(keeper Keeper) sdk.EndBlocker {
	return func(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
		validatorUpdates, _ := EndBlocker(ctx, keeper)

		return abci.ResponseEndBlock{
			ValidatorUpdates: validatorUpdates,
		}
	}
}

// getInitChainer initializes the chainer of the mock app and sets the genesis
// state. It returns an empty ResponseInitChain.
func getInitChainer(mapp *mock.App, keeper Keeper) sdk.InitChainer {
	return func(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
		mapp.InitChainer(ctx, req)

		stakeGenesis := DefaultGenesisState()
		stakeGenesis.Pool.LooseTokens = sdk.NewDecWithoutFra(100000)

		validators, err := InitGenesis(ctx, keeper, stakeGenesis)
		if err != nil {
			panic(err)
		}

		return abci.ResponseInitChain{
			Validators: validators,
		}
	}
}

//__________________________________________________________________________________________

func checkValidator(t *testing.T, mapp *mock.App, keeper Keeper,
	addr sdk.ValAddress, expFound bool) Validator {

	ctxCheck := mapp.BaseApp.NewContext(sdk.RunTxModeCheck, abci.Header{})
	validator, found := keeper.GetValidator(ctxCheck, addr)

	require.Equal(t, expFound, found)
	return validator
}

func checkDelegation(
	t *testing.T, mapp *mock.App, keeper Keeper, delegatorAddr sdk.AccAddress,
	validatorAddr sdk.ValAddress, expFound bool, expShares sdk.Dec,
) {

	ctxCheck := mapp.BaseApp.NewContext(sdk.RunTxModeCheck, abci.Header{})
	delegation, found := keeper.GetDelegation(ctxCheck, delegatorAddr, validatorAddr)
	if expFound {
		require.True(t, found)
		require.True(sdk.DecEq(t, expShares, delegation.Shares))

		return
	}

	require.False(t, found)
}

func TestStakeMsgs(t *testing.T) {
	mApp, keeper := getMockApp(t)

	genCoin := sdk.NewCoin("steak", sdk.NewDecWithoutFra(42).RawInt())
	bondCoin := sdk.NewCoin("steak", sdk.NewDecWithoutFra(10).RawInt())

	acc1 := &auth.BaseAccount{
		Address: addr1,
		Coins:   sdk.Coins{genCoin},
	}
	acc2 := &auth.BaseAccount{
		Address: addr2,
		Coins:   sdk.Coins{genCoin},
	}
	accs := []sdk.Account{acc1, acc2}

	mock.SetGenesis(mApp, accs)
	mock.CheckBalance(t, mApp, addr1, sdk.Coins{genCoin})
	mock.CheckBalance(t, mApp, addr2, sdk.Coins{genCoin})

	// create validator
	description := NewDescription("foo_moniker", "", "", "")
	createValidatorMsg := NewMsgCreateValidator(
		sdk.ValAddress(addr1), priv1.PubKey(), bondCoin, description, commissionMsg,
	)

	mock.SignCheckDeliver(t, mApp.BaseApp, []sdk.Msg{createValidatorMsg}, []int64{0}, []int64{0}, true, true, priv1)
	mock.CheckBalance(t, mApp, addr1, sdk.Coins{genCoin.Minus(bondCoin)})
	mApp.BeginBlock(abci.RequestBeginBlock{})

	validator := checkValidator(t, mApp, keeper, sdk.ValAddress(addr1), true)
	require.Equal(t, sdk.ValAddress(addr1), validator.OperatorAddr)
	require.Equal(t, sdk.Bonded, validator.Status)
	require.True(sdk.DecEq(t, sdk.NewDecWithoutFra(10), validator.BondedTokens()))

	// addr1 create validator on behalf of addr2
	createValidatorMsgOnBehalfOf := NewMsgCreateValidatorOnBehalfOf(
		addr1, sdk.ValAddress(addr2), priv2.PubKey(), bondCoin, description, commissionMsg,
	)

	mock.SignCheckDeliver(t, mApp.BaseApp, []sdk.Msg{createValidatorMsgOnBehalfOf}, []int64{0, 0}, []int64{1, 0}, true, true, priv1, priv2)
	mock.CheckBalance(t, mApp, addr1, sdk.Coins{genCoin.Minus(bondCoin).Minus(bondCoin)})
	mApp.BeginBlock(abci.RequestBeginBlock{})

	validator = checkValidator(t, mApp, keeper, sdk.ValAddress(addr2), true)
	require.Equal(t, sdk.ValAddress(addr2), validator.OperatorAddr)
	require.Equal(t, sdk.Bonded, validator.Status)
	require.True(sdk.DecEq(t, sdk.NewDecWithoutFra(10), validator.Tokens))

	// check the bond that should have been created as well
	checkDelegation(t, mApp, keeper, addr1, sdk.ValAddress(addr1), true, sdk.NewDecWithoutFra(10))

	// edit the validator
	description = NewDescription("bar_moniker", "", "", "")
	editValidatorMsg := NewMsgEditValidator(sdk.ValAddress(addr1), description, nil)

	mock.SignCheckDeliver(t, mApp.BaseApp, []sdk.Msg{editValidatorMsg}, []int64{0}, []int64{2}, true, true, priv1)
	validator = checkValidator(t, mApp, keeper, sdk.ValAddress(addr1), true)
	require.Equal(t, description, validator.Description)

	// delegate
	mock.CheckBalance(t, mApp, addr2, sdk.Coins{genCoin})
	delegateMsg := NewMsgDelegate(addr2, sdk.ValAddress(addr1), bondCoin)

	mock.SignCheckDeliver(t, mApp.BaseApp, []sdk.Msg{delegateMsg}, []int64{0}, []int64{1}, true, true, priv2)
	mock.CheckBalance(t, mApp, addr2, sdk.Coins{genCoin.Minus(bondCoin)})
	checkDelegation(t, mApp, keeper, addr2, sdk.ValAddress(addr1), true, sdk.NewDecWithoutFra(10))

	// begin unbonding
	beginUnbondingMsg := NewMsgBeginUnbonding(addr2, sdk.ValAddress(addr1), sdk.NewDecWithoutFra(10))
	mock.SignCheckDeliver(t, mApp.BaseApp, []sdk.Msg{beginUnbondingMsg}, []int64{0}, []int64{2}, true, true, priv2)

	// delegation should exist anymore
	checkDelegation(t, mApp, keeper, addr2, sdk.ValAddress(addr1), false, sdk.Dec{})

	// balance should be the same because bonding not yet complete
	mock.CheckBalance(t, mApp, addr2, sdk.Coins{genCoin.Minus(bondCoin)})
}
