package stubs

import "uk.ac.bris.cs/gameoflife/gol"

var ReverseHandler = "SecretStringOperations.Reverse"
var PremiumReverseHandler = "SecretStringOperations.FastReverse"

type Response struct {
	updatedWorld [][]uint8
}

type Request struct {
	world [][]uint8
	p     gol.Params
	c     gol.DistributorChannels
}
