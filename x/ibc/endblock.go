package ibc

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func EndBlocker(ctx sdk.Context, keeper Keeper) {
	if keeper.ibcPacakgeCollector == nil {
		return
	}
	var attributes []sdk.Attribute
	for _, ibcPackageRecord := range keeper.ibcPacakgeCollector {
		attributes = append(attributes,
			sdk.NewAttribute(ibcPackageInfoAttributeKey,
				buildIBCPackageAttributeValue(ibcPackageRecord.destChainName, ibcPackageRecord.destChainID, ibcPackageRecord.channelID, ibcPackageRecord.sequence)))
	}
	keeper.ibcPacakgeCollector = nil
	event := sdk.NewEvent(ibcEventType, attributes...)
	ctx.EventManager().EmitEvent(event)
}
