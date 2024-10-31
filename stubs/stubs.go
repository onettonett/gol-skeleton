package stubs

import "uk.ac.bris.cs/gameoflife/gol"

var ReverseHandler = "SecretStringOperations.Reverse"
var PremiumReverseHandler = "SecretStringOperations.FastReverse"

type Response struct {
	UpdatedWorld [][]uint8
}

type Request struct {
	World [][]uint8
	P     gol.Params
	C     gol.DistributorChannels
}
