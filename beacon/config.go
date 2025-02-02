package beacon

import (
	"math/big"
	"time"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

// Config is the configuration of the beacon.
type Config struct {
	// Security parameter (for calculating ATX threshold)
	Kappa int `mapstructure:"beacon-kappa"`
	// Ratio of dishonest spacetime (for calculating ATX threshold). It should be a string representing a rational number.
	Q *big.Rat `mapstructure:"beacon-q"`
	// Amount of rounds in every epoch
	RoundsNumber types.RoundID `mapstructure:"beacon-rounds-number"`
	// Grace period duration
	GracePeriodDuration time.Duration `mapstructure:"beacon-grace-period-duration"`
	// Proposal phase duration
	ProposalDuration time.Duration `mapstructure:"beacon-proposal-duration"`
	// First voting round duration
	FirstVotingRoundDuration time.Duration `mapstructure:"beacon-first-voting-round-duration"`
	// Voting round duration
	VotingRoundDuration time.Duration `mapstructure:"beacon-voting-round-duration"`
	// Weak coin round duration
	WeakCoinRoundDuration time.Duration `mapstructure:"beacon-weak-coin-round-duration"`
	// Ratio of votes for reaching consensus
	Theta *big.Rat `mapstructure:"beacon-theta"`
	// Maximum allowed number of votes to be sent
	VotesLimit uint32 `mapstructure:"beacon-votes-limit"`
	// Numbers of layers to wait before determining beacon values from ballots when the node didn't participate
	// in previous epoch.
	BeaconSyncWeightUnits int `mapstructure:"beacon-sync-weight-units"`
}

// DefaultConfig returns the default configuration for the beacon.
func DefaultConfig() Config {
	return Config{
		Kappa:                    40,
		Q:                        big.NewRat(1, 3),
		RoundsNumber:             300,
		GracePeriodDuration:      2 * time.Minute,
		ProposalDuration:         2 * time.Minute,
		FirstVotingRoundDuration: 1 * time.Hour,
		VotingRoundDuration:      30 * time.Minute,
		WeakCoinRoundDuration:    1 * time.Minute,
		Theta:                    big.NewRat(1, 4),
		VotesLimit:               100, // TODO: around 100, find the calculation in the forum
		BeaconSyncWeightUnits:    800, // at least 1 cluster of 800 weight units
	}
}

// UnitTestConfig returns the unit test configuration for the beacon.
func UnitTestConfig() Config {
	return Config{
		Kappa:                    40,
		Q:                        big.NewRat(1, 3),
		RoundsNumber:             10,
		GracePeriodDuration:      50 * time.Millisecond,
		ProposalDuration:         50 * time.Millisecond,
		FirstVotingRoundDuration: 90 * time.Millisecond,
		VotingRoundDuration:      50 * time.Millisecond,
		WeakCoinRoundDuration:    50 * time.Millisecond,
		Theta:                    big.NewRat(1, 25000),
		VotesLimit:               100,
		BeaconSyncWeightUnits:    2,
	}
}

// NodeSimUnitTestConfig returns configuration for the beacon the unit tests with node simulation .
func NodeSimUnitTestConfig() Config {
	return Config{
		Kappa:                    40,
		Q:                        big.NewRat(1, 3),
		RoundsNumber:             2,
		GracePeriodDuration:      200 * time.Millisecond,
		ProposalDuration:         500 * time.Millisecond,
		FirstVotingRoundDuration: time.Second,
		VotingRoundDuration:      500 * time.Millisecond,
		WeakCoinRoundDuration:    200 * time.Millisecond,
		Theta:                    big.NewRat(1, 25000),
		VotesLimit:               100,
		BeaconSyncWeightUnits:    10,
	}
}
