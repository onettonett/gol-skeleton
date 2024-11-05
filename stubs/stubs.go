package stubs


var ReverseHandler = "SecretStringOperations.Reverse"
var PremiumReverseHandler = "SecretStringOperations.FastReverse"

type Response struct {
	UpdatedWorld [][]uint8
}

type Request struct {
	World [][]uint8
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type AliveCellsCountRequest struct {}

type AliveCellsCountResponse struct {
	CellsAlive int
	Turns int
}

type StateRequest struct {
	Command string
}

type StateResponse struct {
	World [][]uint8
	Turns int
}