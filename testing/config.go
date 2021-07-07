package ibctesting

import (
	connectiontypes "github.com/cosmos/ibc-go/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/modules/core/04-channel/types"

	"github.com/datachainlab/ibc-multisig-client/modules/light-clients/xx-multisig/types"
	"github.com/datachainlab/ibc-multisig-client/testing/mock"
)

type ClientConfig interface {
	GetClientType() string
}

type MultisigConfig struct {
	Diversifier string
	NKeys       uint64
}

func NewMultisigConfig(diversifier string, nKeys uint64) *MultisigConfig {
	return &MultisigConfig{
		Diversifier: diversifier,
		NKeys:       nKeys,
	}
}

func (tmcfg *MultisigConfig) GetClientType() string {
	return types.Multisig
}

type ConnectionConfig struct {
	DelayPeriod uint64
	Version     *connectiontypes.Version
}

func NewConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		DelayPeriod: DefaultDelayPeriod,
		Version:     ConnectionVersion,
	}
}

type ChannelConfig struct {
	PortID  string
	Version string
	Order   channeltypes.Order
}

func NewChannelConfig() *ChannelConfig {
	return &ChannelConfig{
		PortID:  mock.ModuleName,
		Version: DefaultChannelVersion,
		Order:   channeltypes.UNORDERED,
	}
}
