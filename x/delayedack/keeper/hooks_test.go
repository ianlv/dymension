package keeper_test

import (
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"

	"github.com/dymensionxyz/dymension/v3/app/apptesting"
	commontypes "github.com/dymensionxyz/dymension/v3/x/common/types"
	"github.com/dymensionxyz/dymension/v3/x/delayedack/types"
)

// TestAfterEpochEnd tests that the finalized of rollapp packets
// are deleted given the correct epoch identifier
func (suite *DelayedAckTestSuite) TestAfterEpochEnd() {
	tests := []struct {
		name                 string
		pendingPacketsNum    int
		finalizePacketsNum   int
		epochIdentifierParam string
		epochIdentifier      string
		expectedDeleted      int
		expectedTotal        int
	}{
		{
			name:                 "delete rollapp packets after epoch end",
			pendingPacketsNum:    5,
			finalizePacketsNum:   3,
			epochIdentifierParam: "minute",
			epochIdentifier:      "minute",
			expectedDeleted:      3,
			expectedTotal:        2,
		},
		{
			name:                 "fail delete rollapp packets after epoch end - invalid epoch identifier",
			pendingPacketsNum:    5,
			finalizePacketsNum:   3,
			epochIdentifierParam: "minute",
			epochIdentifier:      "hour",
			expectedDeleted:      0,
			expectedTotal:        5,
		},
	}

	const rollappID = "testRollappId"

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			keeper, ctx := suite.App.DelayedAckKeeper, suite.Ctx
			for i := 1; i <= tc.pendingPacketsNum; i++ {
				rollappPacket := &commontypes.RollappPacket{
					RollappId: rollappID,
					Packet: &channeltypes.Packet{
						SourcePort:         "testSourcePort",
						SourceChannel:      "testSourceChannel",
						DestinationPort:    "testDestinationPort",
						DestinationChannel: "testDestinationChannel",
						Data:               apptesting.GenerateTestPacketData(suite.T()),
						Sequence:           uint64(i), //nolint:gosec
					},
					Status:      commontypes.Status_PENDING,
					ProofHeight: uint64(i * 2), //nolint:gosec
				}
				keeper.SetRollappPacket(ctx, *rollappPacket)
			}

			rollappPackets := keeper.ListRollappPackets(ctx, types.ByRollappIDByStatus(rollappID, commontypes.Status_PENDING))
			suite.Require().Equal(tc.pendingPacketsNum, len(rollappPackets))

			for _, rollappPacket := range rollappPackets[:tc.finalizePacketsNum] {
				_, err := keeper.UpdateRollappPacketAfterFinalization(ctx, rollappPacket)
				suite.Require().NoError(err)
			}
			finalizedRollappPackets := keeper.ListRollappPackets(ctx, types.ByRollappIDByStatus(rollappID, commontypes.Status_FINALIZED))
			suite.Require().Equal(tc.finalizePacketsNum, len(finalizedRollappPackets))

			params := keeper.GetParams(ctx)
			keeper.SetParams(ctx, types.Params{
				EpochIdentifier:         tc.epochIdentifierParam,
				BridgingFee:             params.BridgingFee,
				DeletePacketsEpochLimit: params.DeletePacketsEpochLimit,
			})
			epochHooks := keeper.GetEpochHooks()
			err := epochHooks.AfterEpochEnd(ctx, tc.epochIdentifier, 1)
			suite.Require().NoError(err)

			finalizedRollappPackets = keeper.ListRollappPackets(ctx, types.ByRollappIDByStatus(rollappID, commontypes.Status_FINALIZED))
			suite.Require().Equal(tc.finalizePacketsNum-tc.expectedDeleted, len(finalizedRollappPackets))

			pendingPackets := keeper.ListRollappPackets(ctx, types.ByRollappIDByStatus(rollappID, commontypes.Status_PENDING))
			totalRollappPackets := len(finalizedRollappPackets) + len(pendingPackets)
			suite.Require().Equal(tc.expectedTotal, totalRollappPackets)
		})
	}
}
