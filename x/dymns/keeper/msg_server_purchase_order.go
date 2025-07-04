package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/dymensionxyz/gerr-cosmos/gerrc"

	sdk "github.com/cosmos/cosmos-sdk/types"
	dymnstypes "github.com/dymensionxyz/dymension/v3/x/dymns/types"
)

// PurchaseOrder is message handler,
// handles purchasing a Dym-Name/Alias from a Sell-Order, performed by the buyer.
func (k msgServer) PurchaseOrder(goCtx context.Context, msg *dymnstypes.MsgPurchaseOrder) (*dymnstypes.MsgPurchaseOrderResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	originalConsumedGas := ctx.GasMeter().GasConsumed()

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	priceParams := k.PriceParams(ctx)
	miscParams := k.MiscParams(ctx)

	var resp *dymnstypes.MsgPurchaseOrderResponse
	var err error

	// process the purchase order based on the asset type

	switch msg.AssetType {
	case dymnstypes.TypeName:
		resp, err = k.processPurchaseOrderWithAssetTypeDymName(ctx, msg, priceParams, miscParams)
	case dymnstypes.TypeAlias:
		resp, err = k.processPurchaseOrderWithAssetTypeAlias(ctx, msg, priceParams, miscParams)
	default:
		err = errorsmod.Wrapf(gerrc.ErrInvalidArgument, "invalid asset type: %s", msg.AssetType)
	}
	if err != nil {
		return nil, err
	}

	// charge protocol fee
	consumeMinimumGas(ctx, dymnstypes.OpGasPlaceBidOnSellOrder, originalConsumedGas, "PurchaseOrder")

	return resp, nil
}

// processPurchaseOrderWithAssetTypeDymName handles the message handled by PurchaseOrder, type Dym-Name.
func (k msgServer) processPurchaseOrderWithAssetTypeDymName(ctx sdk.Context, msg *dymnstypes.MsgPurchaseOrder, priceParams dymnstypes.PriceParams, miscParams dymnstypes.MiscParams) (*dymnstypes.MsgPurchaseOrderResponse, error) {
	if !miscParams.EnableTradingName {
		return nil, errorsmod.Wrapf(gerrc.ErrFailedPrecondition, "trading of Dym-Name is disabled")
	}

	dymName, so, err := k.validatePurchaseOrderWithAssetTypeDymName(ctx, msg, priceParams)
	if err != nil {
		return nil, err
	}

	if so.HighestBid != nil {
		// refund previous bidder
		if err := k.RefundBid(ctx, *so.HighestBid, so.AssetType); err != nil {
			return nil, err
		}
	}

	// deduct offer price from buyer's account
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		sdk.MustAccAddressFromBech32(msg.Buyer),
		dymnstypes.ModuleName,
		sdk.Coins{msg.Offer},
	); err != nil {
		return nil, err
	}

	// record new highest bid
	so.HighestBid = &dymnstypes.SellOrderBid{
		Bidder: msg.Buyer,
		Price:  msg.Offer,
		Params: msg.Params,
	}

	// after highest bid updated, update SO to store to reflect the new state
	if err := k.SetSellOrder(ctx, *so); err != nil {
		return nil, err
	}

	// try to complete the purchase

	if so.HasFinishedAtCtx(ctx) {
		if err := k.CompleteDymNameSellOrder(ctx, dymName.Name); err != nil {
			return nil, err
		}
	}

	return &dymnstypes.MsgPurchaseOrderResponse{}, nil
}

// validatePurchaseOrderWithAssetTypeDymName handles validation for the message handled by PurchaseOrder, type Dym-Name.
func (k msgServer) validatePurchaseOrderWithAssetTypeDymName(ctx sdk.Context, msg *dymnstypes.MsgPurchaseOrder, priceParams dymnstypes.PriceParams) (*dymnstypes.DymName, *dymnstypes.SellOrder, error) {
	dymName := k.GetDymName(ctx, msg.AssetId)
	if dymName == nil {
		return nil, nil, errorsmod.Wrapf(gerrc.ErrNotFound, "Dym-Name: %s", msg.AssetId)
	}

	if dymName.Owner == msg.Buyer {
		return nil, nil, errorsmod.Wrap(gerrc.ErrPermissionDenied, "cannot purchase your own dym name")
	}

	so := k.GetSellOrder(ctx, msg.AssetId, msg.AssetType)
	if so == nil {
		return nil, nil, errorsmod.Wrapf(gerrc.ErrNotFound, "Sell-Order: %s", msg.AssetId)
	}
	err := k.genericValidateSellOrderOfPurchaseOrder(ctx, msg, *so, priceParams)
	if err != nil {
		return nil, nil, err
	}

	return dymName, so, nil
}

// processPurchaseOrderWithAssetTypeAlias handles the message handled by PurchaseOrder, type Alias.
func (k msgServer) processPurchaseOrderWithAssetTypeAlias(ctx sdk.Context, msg *dymnstypes.MsgPurchaseOrder, priceParams dymnstypes.PriceParams, miscParams dymnstypes.MiscParams) (*dymnstypes.MsgPurchaseOrderResponse, error) {
	if !miscParams.EnableTradingAlias {
		return nil, errorsmod.Wrapf(gerrc.ErrFailedPrecondition, "trading of Alias is disabled")
	}

	so, err := k.validatePurchaseOrderWithAssetTypeAlias(ctx, msg, priceParams)
	if err != nil {
		return nil, err
	}

	if so.HighestBid != nil {
		// refund previous bidder
		if err := k.RefundBid(ctx, *so.HighestBid, so.AssetType); err != nil {
			return nil, err
		}
	}

	// deduct offer price from buyer's account
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		sdk.MustAccAddressFromBech32(msg.Buyer),
		dymnstypes.ModuleName,
		sdk.Coins{msg.Offer},
	); err != nil {
		return nil, err
	}

	// record new highest bid
	so.HighestBid = &dymnstypes.SellOrderBid{
		Bidder: msg.Buyer,
		Price:  msg.Offer,
		Params: msg.Params,
	}

	// after highest bid updated, update SO to store to reflect the new state
	if err := k.SetSellOrder(ctx, *so); err != nil {
		return nil, err
	}

	// try to complete the purchase
	if so.HasFinishedAtCtx(ctx) {
		if err := k.CompleteAliasSellOrder(ctx, so.AssetId); err != nil {
			return nil, err
		}
	}

	return &dymnstypes.MsgPurchaseOrderResponse{}, nil
}

// validatePurchaseOrderWithAssetTypeAlias handles validation for the message handled by PurchaseOrder, type Alias.
func (k msgServer) validatePurchaseOrderWithAssetTypeAlias(ctx sdk.Context, msg *dymnstypes.MsgPurchaseOrder, priceParams dymnstypes.PriceParams) (*dymnstypes.SellOrder, error) {
	destinationRollAppId := msg.Params[0]

	if !k.IsRollAppId(ctx, destinationRollAppId) {
		return nil, errorsmod.Wrapf(gerrc.ErrInvalidArgument, "destination Roll-App does not exists: %s", destinationRollAppId)
	}

	if !k.IsRollAppCreator(ctx, destinationRollAppId, msg.Buyer) {
		return nil, errorsmod.Wrapf(gerrc.ErrPermissionDenied, "not the owner of the RollApp: %s", destinationRollAppId)
	}

	existingRollAppIdUsingAlias, found := k.GetRollAppIdByAlias(ctx, msg.AssetId)
	if !found {
		return nil, errorsmod.Wrapf(gerrc.ErrNotFound, "alias not owned by any RollApp: %s", msg.AssetId)
	}

	if destinationRollAppId == existingRollAppIdUsingAlias {
		return nil, errorsmod.Wrap(gerrc.ErrInvalidArgument, "destination Roll-App ID is the same as the source")
	}

	if k.IsAliasPresentsInParamsAsAliasOrChainId(ctx, msg.AssetId) {
		// Please read the `processCompleteSellOrderWithAssetTypeAlias` method (msg_server_complete_sell_order.go) for more information.
		return nil, errorsmod.Wrapf(gerrc.ErrPermissionDenied,
			"prohibited to trade aliases which is reserved for chain-id or alias in module params: %s", msg.AssetId,
		)
	}

	so := k.GetSellOrder(ctx, msg.AssetId, msg.AssetType)
	if so == nil {
		return nil, errorsmod.Wrapf(gerrc.ErrNotFound, "Sell-Order: %s", msg.AssetId)
	}
	err := k.genericValidateSellOrderOfPurchaseOrder(ctx, msg, *so, priceParams)
	if err != nil {
		return nil, err
	}

	return so, nil
}

// genericValidateSellOrderOfPurchaseOrder is a helper function to validate the purchase order request.
func (k msgServer) genericValidateSellOrderOfPurchaseOrder(ctx sdk.Context, msg *dymnstypes.MsgPurchaseOrder, so dymnstypes.SellOrder, priceParams dymnstypes.PriceParams) error {
	if so.HasExpiredAtCtx(ctx) {
		return errorsmod.Wrap(gerrc.ErrFailedPrecondition, "cannot purchase an expired order")
	}

	if so.HasFinishedAtCtx(ctx) {
		return errorsmod.Wrap(gerrc.ErrFailedPrecondition, "cannot purchase a completed order")
	}

	if msg.Offer.Denom != so.MinPrice.Denom {
		return errorsmod.Wrapf(gerrc.ErrInvalidArgument,
			"offer denom does not match the order denom: %s != %s",
			msg.Offer.Denom, so.MinPrice.Denom,
		)
	}

	if msg.Offer.IsLT(so.MinPrice) {
		return errorsmod.Wrap(gerrc.ErrInvalidArgument, "offer is lower than minimum price")
	}

	if so.HasSetSellPrice() {
		if !msg.Offer.IsLTE(*so.SellPrice) { // overpaid protection
			return errorsmod.Wrap(gerrc.ErrInvalidArgument, "offer is higher than sell price")
		}
	}

	if so.HighestBid != nil {
		if msg.Offer.IsLTE(so.HighestBid.Price) {
			return errorsmod.Wrap(
				gerrc.ErrInvalidArgument,
				"new offer must be higher than current highest bid",
			)
		}

		if priceParams.MinBidIncrementPercent > 0 {
			minimumIncrement := so.HighestBid.Price.Amount.MulRaw(int64(priceParams.MinBidIncrementPercent)).QuoRaw(100)
			if minimumIncrement.IsPositive() {
				wantMinimumBid := so.HighestBid.Price.AddAmount(minimumIncrement)
				if so.HasSetSellPrice() && so.SellPrice.IsLT(wantMinimumBid) {
					// skip the validation
				} else {
					if msg.Offer.IsLT(wantMinimumBid) {
						return errorsmod.Wrapf(
							gerrc.ErrInvalidArgument,
							"new offer must be higher than current highest bid at least %d percent, want minimum offer: %s", priceParams.MinBidIncrementPercent, wantMinimumBid,
						)
					}
				}
			}
		}
	}

	return nil
}
