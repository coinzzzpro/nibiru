package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MaxCommissionRate defines the maximum commission rate that can be set by a validator.
const MaxCommissionRate = "0.25"

var _ sdk.AnteDecorator = (*AnteDecoratorStakingCommission)(nil)

// AnteDecoratorStakingCommission enforces the maximum staking commission for validators.
type AnteDecoratorStakingCommission struct{}

// AnteHandle implements sdk.AnteDecorator. It checks if the transaction involves
// creating or editing a validator with a commission rate higher than the allowed maximum.
func (a AnteDecoratorStakingCommission) AnteHandle(
	ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler,
) (newCtx sdk.Context, err error) {
	maxCommission, err := sdk.NewDecFromStr(MaxCommissionRate)
	if err != nil {
		return ctx, sdk.Wrap(err, "invalid max commission rate")
	}

	for _, msg := range tx.GetMsgs() {
		switch msg := msg.(type) {
		case *stakingtypes.MsgCreateValidator:
			if msg.Commission.Rate.GT(maxCommission) {
				return ctx, NewErrMaxValidatorCommission(msg.Commission.Rate)
			}
		case *stakingtypes.MsgEditValidator:
			if msg.CommissionRate != nil && msg.CommissionRate.GT(maxCommission) {
				return ctx, NewErrMaxValidatorCommission(*msg.CommissionRate)
			}
		default:
			continue
		}
	}

	return next(ctx, tx, simulate)
}
