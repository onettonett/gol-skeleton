package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	"fmt"
)

// distributor.go acts as the client
// server file is on the server
// go run server/server.go
// pressed green button in distributor

// Secret method that we can't let clients see
func nextState(world [][]uint8, turns int, threads int, imageWidth int, imageHeight int) [][]uint8 {

	H := imageHeight
	W := imageWidth

	// make toReturn 2d slice
	toReturn := make([][]uint8, H) // create a slice with 16 rows
	for i := 0; i < H; i++ {
        toReturn[i] = make([]uint8, W)
        // Copy the initial world state instead of reading from channel
        copy(toReturn[i], world[i])
    }

	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			sum := 0
			if world[y%H][(x-1+W)%W] != 0 {
				sum += 1
			}
			if world[y%H][(x+1)%W] != 0 {
				sum += 1
			}
			if world[(y+1)%H][x%W] != 0 {
				sum += 1
			}
			if world[(y+1)%H][(x+1)%W] != 0 {
				sum += 1
			}
			if world[(y+1)%H][(x-1+W)%W] != 0 {
				sum += 1
			}
			if world[(y-1+H)%H][x%W] != 0 {
				sum += 1
			}
			if world[(y-1+H)%H][(x+1)%W] != 0 {
				sum += 1
			}
			if world[(y-1+H)%H][(x-1+W)%W] != 0 {
				sum += 1
			}

			if world[y][x] == 255 {
				// the cell was previously alive
				if sum < 2 || sum > 3 {
					toReturn[y][x] = 0
				} else if sum == 2 || sum == 3 {
					// keep the cell alive
					toReturn[y][x] = 255
				}
			} else if world[y][x] == 0 {
				// the cell was previously dead
				if sum == 3 {
					toReturn[y][x] = 255
				} else {
					toReturn[y][x] = 0
				}
			}
		}
	}
	return toReturn
}

func doAllTurns(world [][]uint8, turns int, threads int, imageWidth int, imageHeight int, aliveCellsChan chan chan GameState, worldStateChan chan chan WorldState) [][]uint8 {
	for t := 0; t < turns; t++ {
        select {
        case responseChan := <-aliveCellsChan:
            // When we receive a request for the alive cells count,
            // calculate and send it through the response channel
			fmt.Printf("AliveCellsCount request received in doAllTurns\n")
            state := GameState{
                AliveCells: len(calculateAliveCells(world, imageWidth, imageHeight)),
                CurrentTurn: t,
            }
			fmt.Printf("Count: %d\n", state.AliveCells)
			fmt.Printf("Turn: %d\n", state.CurrentTurn)
            responseChan <- state
		case responseChan := <-worldStateChan:
            state := WorldState{
                World: world,
                CurrentTurn: t,
            }
            responseChan <- state
        default:
            // Continue with normal game processing
			world = nextState(world, turns, threads, imageWidth, imageHeight)
            // ... rest of the loop code ...
        }
    }
	return world
}

type SecretStringOperations struct{
	aliveCellsChannel chan chan GameState
	worldStateChannel chan chan WorldState
}

type GameState struct {
    AliveCells int
    CurrentTurn int
}

type WorldState struct {
    World       [][]uint8
    CurrentTurn int
}

// this is like the Reverse method in SecretStrings
func (s *SecretStringOperations) Start(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Printf("Received request: %v\n", req)
	s.aliveCellsChannel = make(chan chan GameState)
	s.worldStateChannel = make(chan chan WorldState)
	res.UpdatedWorld = doAllTurns(req.World, req.Turns, req.Threads, req.ImageWidth, req.ImageHeight, s.aliveCellsChannel, s.worldStateChannel)
	// func nextState(world [][]uint8, p gol.Params, c gol.DistributorChannels) [][]uint8
	return nil
}


func (s *SecretStringOperations) AliveCellsCount(req stubs.AliveCellsCountRequest, res *stubs.AliveCellsCountResponse) (err error) {
	fmt.Printf("AliveCellsCount request received\n")
	responseChannel := make(chan GameState)
	s.aliveCellsChannel <- responseChannel
	fmt.Printf("Received request: %v\n", req)
	state := <-responseChannel
	res.CellsAlive = state.AliveCells
	res.Turns = state.CurrentTurn
	// func nextState(world [][]uint8, p gol.Params, c gol.DistributorChannels) [][]uint8
	return nil
}




func (s *SecretStringOperations) State(req stubs.StateRequest, res *stubs.StateResponse) (err error) {
	fmt.Printf("Received state request: %v\n", req.Command)
	switch req.Command {
		case "save":
			// TODO: Implement save functionality
			fmt.Println("Save command received")
			worldStateChannel := make(chan WorldState)
			s.worldStateChannel <- worldStateChannel
			worldState := <-worldStateChannel
			res.World = worldState.World
			res.Turns = worldState.CurrentTurn
		case "quit":
			// TODO: Implement quit functionality 
			fmt.Println("Quit command received")
		case "pause":
			// TODO: Implement pause functionality
			fmt.Println("Pause command received") 
		default:
			fmt.Printf("Unknown command: %s\n", req.Command)
	}
	return nil
}

func calculateAliveCells(world [][]byte, imageWidth int, imageHeight int) []util.Cell {
	alives := make([]util.Cell, 0)
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			if world[y][x] == 255 {
				newCell := util.Cell{x, y}
				alives = append(alives, newCell)
			}
		}
	}
	return alives
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&SecretStringOperations{})
	listener, _ := net.Listen("tcp", "localhost:8030")
	defer listener.Close()
	fmt.Printf("Server is listening on port %s...\n", *pAddr)
	rpc.Accept(listener)
}
