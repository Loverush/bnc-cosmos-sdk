package gov_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/mock"
)

var (
	coinsPos         = sdk.Coins{sdk.NewCoin(gov.DefaultDepositDenom, 1000)}
	coinsZero        = sdk.Coins{}
	coinsNeg         = sdk.Coins{sdk.NewCoin(gov.DefaultDepositDenom, -10000)}
	coinsPosNotAtoms = sdk.Coins{sdk.NewCoin("foo", 10000)}
	coinsMulti       = sdk.Coins{sdk.NewCoin("foo", 10000), sdk.NewCoin(gov.DefaultDepositDenom, 1000)}
)

// test ValidateBasic for MsgCreateValidator
func TestMsgSubmitProposal(t *testing.T) {
	_, addrs, _, _ := mock.CreateGenAccounts(1, sdk.Coins{})
	tests := []struct {
		title, description string
		proposalType       gov.ProposalKind
		proposerAddr       sdk.AccAddress
		initialDeposit     sdk.Coins
		expectPass         bool
	}{
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeText, addrs[0], coinsPos, true},
		{"", "the purpose of this proposal is to test", gov.ProposalTypeText, addrs[0], coinsPos, false},
		{"Test Proposal", "", gov.ProposalTypeText, addrs[0], coinsPos, false},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeParameterChange, addrs[0], coinsPos, true},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeSoftwareUpgrade, addrs[0], coinsPos, true},
		{"Test Proposal", "the purpose of this proposal is to test", 0x05, addrs[0], coinsPos, false},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeText, sdk.AccAddress{}, coinsPos, false},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeText, addrs[0], coinsZero, true},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeText, addrs[0], coinsNeg, false},
		{"Test Proposal", "the purpose of this proposal is to test", gov.ProposalTypeText, addrs[0], coinsMulti, true},
	}

	for i, tc := range tests {
		msg := gov.NewMsgSubmitProposal(tc.title, tc.description, tc.proposalType, tc.proposerAddr, tc.initialDeposit)
		if tc.expectPass {
			require.Nil(t, msg.ValidateBasic(), "test: %v", i)
		} else {
			require.NotNil(t, msg.ValidateBasic(), "test: %v", i)
		}
	}
}

// test ValidateBasic for MsgDeposit
func TestMsgDeposit(t *testing.T) {
	_, addrs, _, _ := mock.CreateGenAccounts(1, sdk.Coins{})
	tests := []struct {
		proposalID    int64
		depositerAddr sdk.AccAddress
		depositAmount sdk.Coins
		expectPass    bool
	}{
		{0, addrs[0], coinsPos, true},
		{-1, addrs[0], coinsPos, false},
		{1, sdk.AccAddress{}, coinsPos, false},
		{1, addrs[0], coinsZero, true},
		{1, addrs[0], coinsNeg, false},
		{1, addrs[0], coinsMulti, true},
	}

	for i, tc := range tests {
		msg := gov.NewMsgDeposit(tc.depositerAddr, tc.proposalID, tc.depositAmount)
		if tc.expectPass {
			require.Nil(t, msg.ValidateBasic(), "test: %v", i)
		} else {
			require.NotNil(t, msg.ValidateBasic(), "test: %v", i)
		}
	}
}

// test ValidateBasic for MsgDeposit
func TestMsgVote(t *testing.T) {
	_, addrs, _, _ := mock.CreateGenAccounts(1, sdk.Coins{})
	tests := []struct {
		proposalID int64
		voterAddr  sdk.AccAddress
		option     gov.VoteOption
		expectPass bool
	}{
		{0, addrs[0], gov.OptionYes, true},
		{-1, addrs[0], gov.OptionYes, false},
		{0, sdk.AccAddress{}, gov.OptionYes, false},
		{0, addrs[0], gov.OptionNo, true},
		{0, addrs[0], gov.OptionNoWithVeto, true},
		{0, addrs[0], gov.OptionAbstain, true},
		{0, addrs[0], gov.VoteOption(0x13), false},
	}

	for i, tc := range tests {
		msg := gov.NewMsgVote(tc.voterAddr, tc.proposalID, tc.option)
		if tc.expectPass {
			require.Nil(t, msg.ValidateBasic(), "test: %v", i)
		} else {
			require.NotNil(t, msg.ValidateBasic(), "test: %v", i)
		}
	}
}
