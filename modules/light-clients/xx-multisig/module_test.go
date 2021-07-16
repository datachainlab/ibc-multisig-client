package multisig_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	ibctesting "github.com/datachainlab/ibc-multisig-client/testing"
)

type MultisigTestSuite struct {
	suite.Suite

	coordinator *ibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA ibctesting.TestChainI
	chainB ibctesting.TestChainI
}

func (suite *MultisigTestSuite) SetupTest() {
	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 2)
	suite.chainA = suite.coordinator.GetChain(ibctesting.GetChainID(0))
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(1))
}

func NewTransferPath(chainA, chainB ibctesting.TestChainI) *ibctesting.Path {
	endpointA := ibctesting.NewEndpoint(
		chainA,
		ibctesting.NewMultisigConfig(
			fmt.Sprintf("%s-%s", chainA.GetChainID(), chainB.GetChainID()),
			2,
		),
		ibctesting.NewConnectionConfig(),
		ibctesting.NewChannelConfig(),
	)

	endpointB := ibctesting.NewEndpoint(
		chainB,
		ibctesting.NewMultisigConfig(
			fmt.Sprintf("%s-%s", chainB.GetChainID(), chainA.GetChainID()),
			2,
		),
		ibctesting.NewConnectionConfig(),
		ibctesting.NewChannelConfig(),
	)

	endpointA.ChannelConfig.PortID = ibctesting.TransferPort
	endpointB.ChannelConfig.PortID = ibctesting.TransferPort

	endpointA.Counterparty = endpointB
	endpointB.Counterparty = endpointA

	return &ibctesting.Path{
		EndpointA: endpointA,
		EndpointB: endpointB,
	}
}

// constructs a send from chainA to chainB on the established channel/connection
// and sends the same coin back from chainB to chainA.
func (suite *MultisigTestSuite) TestHandleMsgTransfer() {
	// setup between chainA and chainB
	path := NewTransferPath(suite.chainA, suite.chainB)
	suite.coordinator.Setup(path)

	//	originalBalance := suite.chainA.GetSimApp().BankKeeper.GetBalance(suite.chainA.GetContext(), suite.chainA.SenderAccount.GetAddress(), sdk.DefaultBondDenom)
	//timeoutHeight := clienttypes.NewHeight(100, 1)
	//
	//coinToSendToB := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(100))
	//
	//// send from chainA to chainB
	//msg := types.NewMsgTransfer(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, coinToSendToB, suite.chainA.GetSenderAccount().GetAddress().String(), suite.chainB.GetSenderAccount().GetAddress().String(), timeoutHeight, 0)
	//
	//_, err := suite.chainA.SendMsgs(msg)
	//suite.Require().NoError(err) // message committed
	//
	//// relay send
	//fungibleTokenPacket := types.NewFungibleTokenPacketData(coinToSendToB.Denom, coinToSendToB.Amount.Uint64(), suite.chainA.GetSenderAccount().GetAddress().String(), suite.chainB.GetSenderAccount().GetAddress().String())
	//packet := channeltypes.NewPacket(fungibleTokenPacket.GetBytes(), 1, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, timeoutHeight, 0)
	//ack := channeltypes.NewResultAcknowledgement([]byte{byte(1)})
	//err = path.RelayPacket(packet, ack.Acknowledgement())
	//suite.Require().NoError(err) // relay committed
	//
	//// check that voucher exists on chain B
	//voucherDenomTrace := types.ParseDenomTrace(types.GetPrefixedDenom(packet.GetDestPort(), packet.GetDestChannel(), sdk.DefaultBondDenom))
	//balance := suite.chainB.GetSimApp().BankKeeper.GetBalance(suite.chainB.GetContext(), suite.chainB.GetSenderAccount().GetAddress(), voucherDenomTrace.IBCDenom())
	//
	//coinSentFromAToB := types.GetTransferCoin(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, sdk.DefaultBondDenom, 100)
	//suite.Require().Equal(coinSentFromAToB, balance)

	//// setup between chainB to chainC
	//// NOTE:
	//// pathBtoC.EndpointA = endpoint on chainB
	//// pathBtoC.EndpointB = endpoint on chainC
	//pathBtoC := NewTransferPath(suite.chainB, suite.chainC)
	//suite.coordinator.Setup(pathBtoC)
	//
	//// send from chainB to chainC
	//msg = types.NewMsgTransfer(pathBtoC.EndpointA.ChannelConfig.PortID, pathBtoC.EndpointA.ChannelID, coinSentFromAToB, suite.chainB.SenderAccount.GetAddress().String(), suite.chainC.SenderAccount.GetAddress().String(), timeoutHeight, 0)
	//
	//_, err = suite.chainB.SendMsgs(msg)
	//suite.Require().NoError(err) // message committed
	//
	//// relay send
	//// NOTE: fungible token is prefixed with the full trace in order to verify the packet commitment
	//fullDenomPath := types.GetPrefixedDenom(pathBtoC.EndpointB.ChannelConfig.PortID, pathBtoC.EndpointB.ChannelID, voucherDenomTrace.GetFullDenomPath())
	//fungibleTokenPacket = types.NewFungibleTokenPacketData(voucherDenomTrace.GetFullDenomPath(), coinSentFromAToB.Amount.Uint64(), suite.chainB.SenderAccount.GetAddress().String(), suite.chainC.SenderAccount.GetAddress().String())
	//packet = channeltypes.NewPacket(fungibleTokenPacket.GetBytes(), 1, pathBtoC.EndpointA.ChannelConfig.PortID, pathBtoC.EndpointA.ChannelID, pathBtoC.EndpointB.ChannelConfig.PortID, pathBtoC.EndpointB.ChannelID, timeoutHeight, 0)
	//err = pathBtoC.RelayPacket(packet, ack.Acknowledgement())
	//suite.Require().NoError(err) // relay committed
	//
	//coinSentFromBToC := sdk.NewInt64Coin(types.ParseDenomTrace(fullDenomPath).IBCDenom(), 100)
	//balance = suite.chainC.GetSimApp().BankKeeper.GetBalance(suite.chainC.GetContext(), suite.chainC.SenderAccount.GetAddress(), coinSentFromBToC.Denom)
	//
	//// check that the balance is updated on chainC
	//suite.Require().Equal(coinSentFromBToC, balance)
	//
	//// check that balance on chain B is empty
	//balance = suite.chainB.GetSimApp().BankKeeper.GetBalance(suite.chainB.GetContext(), suite.chainB.SenderAccount.GetAddress(), coinSentFromBToC.Denom)
	//suite.Require().Zero(balance.Amount.Int64())
	//
	//// send from chainC back to chainB
	//msg = types.NewMsgTransfer(pathBtoC.EndpointB.ChannelConfig.PortID, pathBtoC.EndpointB.ChannelID, coinSentFromBToC, suite.chainC.SenderAccount.GetAddress().String(), suite.chainB.SenderAccount.GetAddress().String(), timeoutHeight, 0)
	//
	//_, err = suite.chainC.SendMsgs(msg)
	//suite.Require().NoError(err) // message committed
	//
	//// relay send
	//// NOTE: fungible token is prefixed with the full trace in order to verify the packet commitment
	//fungibleTokenPacket = types.NewFungibleTokenPacketData(fullDenomPath, coinSentFromBToC.Amount.Uint64(), suite.chainC.SenderAccount.GetAddress().String(), suite.chainB.SenderAccount.GetAddress().String())
	//packet = channeltypes.NewPacket(fungibleTokenPacket.GetBytes(), 1, pathBtoC.EndpointB.ChannelConfig.PortID, pathBtoC.EndpointB.ChannelID, pathBtoC.EndpointA.ChannelConfig.PortID, pathBtoC.EndpointA.ChannelID, timeoutHeight, 0)
	//err = pathBtoC.RelayPacket(packet, ack.Acknowledgement())
	//suite.Require().NoError(err) // relay committed
	//
	//balance = suite.chainB.GetSimApp().BankKeeper.GetBalance(suite.chainB.GetContext(), suite.chainB.SenderAccount.GetAddress(), coinSentFromAToB.Denom)
	//
	//// check that the balance on chainA returned back to the original state
	//suite.Require().Equal(coinSentFromAToB, balance)
	//
	//// check that module account escrow address is empty
	//escrowAddress := types.GetEscrowAddress(packet.GetDestPort(), packet.GetDestChannel())
	//balance = suite.chainB.GetSimApp().BankKeeper.GetBalance(suite.chainB.GetContext(), escrowAddress, sdk.DefaultBondDenom)
	//suite.Require().Equal(sdk.NewCoin(sdk.DefaultBondDenom, sdk.ZeroInt()), balance)
	//
	//// check that balance on chain B is empty
	//balance = suite.chainC.GetSimApp().BankKeeper.GetBalance(suite.chainC.GetContext(), suite.chainC.SenderAccount.GetAddress(), voucherDenomTrace.IBCDenom())
	//suite.Require().Zero(balance.Amount.Int64())
}

func TestMultisigTestSuite(t *testing.T) {
	suite.Run(t, new(MultisigTestSuite))
}
