package keeper

import (
	"fmt"
	"time"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/vpool/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) updateReserve(
	ctx sdk.Context,
	pool *types.Pool,
	dir types.Direction,
	quoteAssetAmount sdk.Dec,
	baseAssetAmount sdk.Dec,
	skipFluctuationCheck bool,
) error {
	if dir == types.Direction_ADD_TO_POOL {
		pool.IncreaseQuoteAssetReserve(quoteAssetAmount)
		pool.DecreaseBaseAssetReserve(baseAssetAmount)
		// TODO baseAssetDeltaThisFunding
		// TODO totalPositionSize
		// TODO cumulativeNotional
	} else {
		pool.DecreaseQuoteAssetReserve(quoteAssetAmount)
		pool.IncreaseBaseAssetReserve(baseAssetAmount)
		// TODO baseAssetDeltaThisFunding
		// TODO totalPositionSize
		// TODO cumulativeNotional
	}

	// Check if its over Fluctuation Limit Ratio.
	if !skipFluctuationCheck {
		err := k.checkFluctuationLimitRatio(ctx, pool)
		if err != nil {
			return err
		}
	}

	err := k.addReserveSnapshot(ctx, common.TokenPair(pool.Pair), pool.QuoteAssetReserve, pool.BaseAssetReserve)
	if err != nil {
		return fmt.Errorf("error creating snapshot: %w", err)
	}

	k.savePool(ctx, pool)

	return nil
}

// addReserveSnapshot adds a snapshot of the current pool status and blocktime and blocknum.
func (k Keeper) addReserveSnapshot(
	ctx sdk.Context,
	pair common.TokenPair,
	quoteAssetReserve sdk.Dec,
	baseAssetReserve sdk.Dec,
) error {
	lastSnapshot, lastCounter, err := k.getLatestReserveSnapshot(ctx, pair)
	if err != nil {
		return err
	}

	if ctx.BlockHeight() == lastSnapshot.BlockNumber {
		k.saveSnapshot(ctx, pair, lastCounter, quoteAssetReserve, baseAssetReserve, ctx.BlockTime(), ctx.BlockHeight())
	} else {
		newCounter := lastCounter + 1
		k.saveSnapshot(ctx, pair, newCounter, quoteAssetReserve, baseAssetReserve, ctx.BlockTime(), ctx.BlockHeight())
		k.saveSnapshotCounter(ctx, pair, newCounter)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventSnapshotSaved,
			sdk.NewAttribute(types.AttributeBlockHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
			sdk.NewAttribute(types.AttributeQuoteReserve, quoteAssetReserve.String()),
			sdk.NewAttribute(types.AttributeBaseReserve, baseAssetReserve.String()),
		),
	)

	return nil
}

// getSnapshot returns the snapshot saved by counter num
func (k Keeper) getSnapshot(ctx sdk.Context, pair common.TokenPair, counter uint64) (
	snapshot types.ReserveSnapshot, err error,
) {
	bz := ctx.KVStore(k.storeKey).Get(types.GetSnapshotKey(pair, counter))
	if bz == nil {
		return types.ReserveSnapshot{}, types.ErrNoLastSnapshotSaved.
			Wrap(fmt.Sprintf("snapshot with counter %d was not found", counter))
	}

	k.codec.MustUnmarshal(bz, &snapshot)

	return snapshot, nil
}

func (k Keeper) saveSnapshot(
	ctx sdk.Context,
	pair common.TokenPair,
	counter uint64,
	quoteAssetReserve sdk.Dec,
	baseAssetReserve sdk.Dec,
	timestamp time.Time,
	blockNumber int64,

) {
	snapshot := &types.ReserveSnapshot{
		BaseAssetReserve:  baseAssetReserve,
		QuoteAssetReserve: quoteAssetReserve,
		TimestampMs:       timestamp.UnixMilli(),
		BlockNumber:       blockNumber,
	}

	ctx.KVStore(k.storeKey).Set(
		types.GetSnapshotKey(pair, counter),
		k.codec.MustMarshal(snapshot),
	)
}

// getSnapshotCounter returns the counter and if it has been found or not.
func (k Keeper) getSnapshotCounter(ctx sdk.Context, pair common.TokenPair) (
	snapshotCounter uint64, found bool,
) {
	bz := ctx.KVStore(k.storeKey).Get(types.GetSnapshotCounterKey(pair))
	if bz == nil {
		return uint64(0), false
	}

	return sdk.BigEndianToUint64(bz), true
}

func (k Keeper) saveSnapshotCounter(
	ctx sdk.Context,
	pair common.TokenPair,
	counter uint64,
) {
	ctx.KVStore(k.storeKey).Set(
		types.GetSnapshotCounterKey(pair),
		sdk.Uint64ToBigEndian(counter),
	)
}

// getLatestReserveSnapshot returns the last snapshot that was saved
func (k Keeper) getLatestReserveSnapshot(ctx sdk.Context, pair common.TokenPair) (
	snapshot types.ReserveSnapshot, counter uint64, err error,
) {
	counter, found := k.getSnapshotCounter(ctx, pair)
	if !found {
		return types.ReserveSnapshot{}, counter, types.ErrNoLastSnapshotSaved
	}

	snapshot, err = k.getSnapshot(ctx, pair, counter)
	if err != nil {
		return types.ReserveSnapshot{}, counter, types.ErrNoLastSnapshotSaved
	}

	return snapshot, counter, nil
}

/*
An object parameter for getPriceWithSnapshot().

Specifies how to read the price from a single snapshot. There are three ways:
SPOT: spot price
QUOTE_ASSET_SWAP: price when swapping x amount of quote assets
BASE_ASSET_SWAP: price when swapping y amount of base assets
*/
type snapshotPriceOptions struct {
	// required
	pair           common.TokenPair
	twapCalcOption types.TwapCalcOption

	// required only if twapCalcOption == QUOTE_ASSET_SWAP or BASE_ASSET_SWAP
	direction   types.Direction
	assetAmount sdk.Dec
}

/*
Pure function that returns a price from a snapshot.

Can choose from three types of calc options: SPOT, QUOTE_ASSET_SWAP, and BASE_ASSET_SWAP.
QUOTE_ASSET_SWAP and BASE_ASSET_SWAP require the `direction`` and `assetAmount`` args.
SPOT does not require `direction` and `assetAmount`.

args:
  - pair: the token pair
  - snapshot: a reserve snapshot
  - twapCalcOption: SPOT, QUOTE_ASSET_SWAP, or BASE_ASSET_SWAP
  - direction: add or remove; only required for QUOTE_ASSET_SWAP or BASE_ASSET_SWAP
  - assetAmount: the amount of base or quote asset; only required for QUOTE_ASSET_SWAP or BASE_ASSET_SWAP

ret:
  - price: the price as sdk.Dec
  - err: error
*/
func getPriceWithSnapshot(
	snapshot types.ReserveSnapshot,
	snapshotPriceOpts snapshotPriceOptions,
) (price sdk.Dec, err error) {
	switch snapshotPriceOpts.twapCalcOption {
	case types.TwapCalcOption_SPOT:
		return snapshot.QuoteAssetReserve.Quo(snapshot.BaseAssetReserve), nil

	case types.TwapCalcOption_QUOTE_ASSET_SWAP:
		pool := types.NewPool(
			snapshotPriceOpts.pair.String(),
			sdk.OneDec(),
			snapshot.QuoteAssetReserve,
			snapshot.BaseAssetReserve,
			sdk.OneDec(),
		)
		return pool.GetBaseAmountByQuoteAmount(snapshotPriceOpts.direction, snapshotPriceOpts.assetAmount)

	case types.TwapCalcOption_BASE_ASSET_SWAP:
		pool := types.NewPool(
			snapshotPriceOpts.pair.String(),
			sdk.OneDec(),
			snapshot.QuoteAssetReserve,
			snapshot.BaseAssetReserve,
			sdk.OneDec(),
		)
		return pool.GetQuoteAmountByBaseAmount(snapshotPriceOpts.direction, snapshotPriceOpts.assetAmount)
	}

	return sdk.ZeroDec(), nil
}