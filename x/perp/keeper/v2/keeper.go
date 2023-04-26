package keeper

import (
	"fmt"

	"github.com/NibiruChain/collections"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/perp/types"
	v2types "github.com/NibiruChain/nibiru/x/perp/types/v2"
)

type Keeper struct {
	cdc           codec.BinaryCodec
	storeKey      sdk.StoreKey
	ParamSubspace paramtypes.Subspace

	BankKeeper    types.BankKeeper
	AccountKeeper types.AccountKeeper
	OracleKeeper  types.OracleKeeper
	PerpAmmKeeper types.PerpAmmKeeper
	EpochKeeper   types.EpochKeeper

	Markets   collections.Map[asset.Pair, v2types.Market]
	AMMs      collections.Map[asset.Pair, v2types.AMM]
	Positions collections.Map[collections.Pair[asset.Pair, sdk.AccAddress], v2types.Position]
	Metrics   collections.Map[asset.Pair, v2types.Metrics]

	// TODO(k-yang): get rid of these collections
	PairsMetadata  collections.Map[asset.Pair, types.PairMetadata]
	PrepaidBadDebt collections.Map[string, types.PrepaidBadDebt]
}

// NewKeeper Creates a new x/perp Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey sdk.StoreKey,
	paramSubspace paramtypes.Subspace,

	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	oracleKeeper types.OracleKeeper,
	perpammKeeper types.PerpAmmKeeper,
	epochKeeper types.EpochKeeper,
) Keeper {
	// Ensure that the module account is set.
	if moduleAcc := accountKeeper.GetModuleAddress(types.ModuleName); moduleAcc == nil {
		panic("The x/perp module account has not been set")
	}

	// Set param.types.'KeyTable' if it has not already been set
	if !paramSubspace.HasKeyTable() {
		paramSubspace = paramSubspace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		ParamSubspace: paramSubspace,
		BankKeeper:    bankKeeper,
		AccountKeeper: accountKeeper,
		OracleKeeper:  oracleKeeper,
		PerpAmmKeeper: perpammKeeper,
		EpochKeeper:   epochKeeper,
		Positions: collections.NewMap(
			storeKey, 0,
			collections.PairKeyEncoder(asset.PairKeyEncoder, collections.AccAddressKeyEncoder),
			collections.ProtoValueEncoder[v2types.Position](cdc),
		),
		PairsMetadata:  collections.NewMap(storeKey, 1, asset.PairKeyEncoder, collections.ProtoValueEncoder[types.PairMetadata](cdc)),
		PrepaidBadDebt: collections.NewMap(storeKey, 2, collections.StringKeyEncoder, collections.ProtoValueEncoder[types.PrepaidBadDebt](cdc)),

		Metrics: collections.NewMap(storeKey, 3, asset.PairKeyEncoder, collections.ProtoValueEncoder[v2types.Metrics](cdc)),
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetParams get all parameters as types.Params
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.ParamSubspace.GetParamSet(ctx, &params)
	return params
}

// SetParams set the params
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.ParamSubspace.SetParamSet(ctx, &params)
}