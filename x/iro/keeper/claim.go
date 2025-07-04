package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/gerr-cosmos/gerrc"
	"github.com/dymensionxyz/sdk-utils/utils/uevent"

	"github.com/dymensionxyz/dymension/v3/x/iro/types"
)

// Claim claims the FUT token for the real RA token
//
// This function allows a user to claim their RA tokens by burning their FUT tokens.
// It burns *all* the FUT tokens the claimer has, and sends the equivalent amount of RA tokens to the claimer.
func (k Keeper) Claim(ctx sdk.Context, planId string, claimer sdk.AccAddress) error {
	plan, found := k.GetPlan(ctx, planId)
	if !found {
		return types.ErrPlanNotFound
	}

	if !plan.IsSettled() {
		return types.ErrPlanNotSettled
	}

	availableTokens := k.BK.GetBalance(ctx, claimer, plan.TotalAllocation.Denom)
	if availableTokens.IsZero() {
		return types.ErrNoTokensToClaim
	}

	// Burn all the FUT tokens the user have
	err := k.BK.SendCoinsFromAccountToModule(ctx, claimer, types.ModuleName, sdk.NewCoins(availableTokens))
	if err != nil {
		return err
	}
	err = k.BK.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(availableTokens))
	if err != nil {
		return err
	}

	// Give the user the RA token in return (same amount as the FUT token)
	err = k.BK.SendCoinsFromModuleToAccount(ctx, types.ModuleName, claimer, sdk.NewCoins(sdk.NewCoin(plan.SettledDenom, availableTokens.Amount)))
	if err != nil {
		return err
	}

	// Update the plan
	plan.ClaimedAmt = plan.ClaimedAmt.Add(availableTokens.Amount)
	k.SetPlan(ctx, plan)

	// Emit event
	err = uevent.EmitTypedEvent(ctx, &types.EventClaim{
		Claimer:   claimer.String(),
		PlanId:    planId,
		RollappId: plan.RollappId,
		Claim:     sdk.NewCoin(plan.SettledDenom, availableTokens.Amount),
	})
	if err != nil {
		return err
	}

	return nil
}

// ClaimVested allows the owner of a RollApp to claim vested tokens.
// The function performs the following checks and operations:
// - Verifies that the plan exists and is settled.
// - Ensures the claimer is the owner of the RollApp.
// - Checks if there are any vested funds available.
// - Transfers the vested tokens from the module account to the claimer's account.
// - Updates the vesting plan to reflect the collected amount.
func (k Keeper) ClaimVested(ctx sdk.Context, planId string, claimer sdk.AccAddress) error {
	plan, found := k.GetPlan(ctx, planId)
	if !found {
		return types.ErrPlanNotFound
	}

	if !plan.IsSettled() {
		return types.ErrPlanNotSettled
	}

	// make sure it's the rollapp owner
	owner := k.rk.MustGetRollappOwner(ctx, plan.RollappId)
	if !owner.Equals(claimer) {
		return errorsmod.Wrap(gerrc.ErrPermissionDenied, "not the owner of the RollApp")
	}

	// check for vested funds
	amt := plan.VestingPlan.VestedAmt(ctx.BlockTime())
	if amt.IsZero() {
		return errorsmod.Wrap(gerrc.ErrFailedPrecondition, "no vested funds")
	}

	// send the vested funds to the claimer
	vestedCoins := sdk.NewCoins(sdk.NewCoin(plan.LiquidityDenom, amt))
	err := k.BK.SendCoins(ctx, plan.GetAddress(), claimer, vestedCoins)
	if err != nil {
		return err
	}

	// update the vesting plan
	plan.VestingPlan.Claimed = plan.VestingPlan.Claimed.Add(amt)
	k.SetPlan(ctx, plan)

	err = uevent.EmitTypedEvent(ctx, &types.EventClaimVested{
		Claimer:   claimer.String(),
		PlanId:    planId,
		RollappId: plan.RollappId,
		Claim:     sdk.NewCoin(plan.LiquidityDenom, amt),
		Unvested:  sdk.NewCoin(plan.LiquidityDenom, plan.VestingPlan.Amount.Sub(plan.VestingPlan.Claimed)),
	})
	if err != nil {
		return err
	}

	return nil
}
