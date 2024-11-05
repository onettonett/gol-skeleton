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
	"os"
)

// distributor.go acts as the client
// server file is on the server
// go run server/server.go
// pressed green button in distributor

var listener net.Listener


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

func doAllTurns(world [][]uint8, turns int, threads int, imageWidth int, imageHeight int, aliveCellsChan chan chan GameState, worldStateChan chan chan WorldState, stopChannel chan bool, pauseChannel chan bool) [][]uint8 {
	for t := 0; t < turns; t++ {
        select {
		case shouldPause := <-pauseChannel:
			if shouldPause {
				fmt.Printf("Waiting for unpause\n")
				<-pauseChannel
			}
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
		case <-stopChannel:
			fmt.Printf("Stopping game\n")
			return world
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
	stopChannel chan bool
	pauseChannel chan bool
	isPaused bool
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
	s.stopChannel = make(chan bool)
	s.pauseChannel = make(chan bool)
	s.isPaused = false
	res.UpdatedWorld = doAllTurns(req.World, req.Turns, req.Threads, req.ImageWidth, req.ImageHeight, s.aliveCellsChannel, s.worldStateChannel, s.stopChannel, s.pauseChannel)
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
			s.stopChannel <- true
		case "pause":
			fmt.Println("Pause command received")
			s.isPaused = !s.isPaused
			s.pauseChannel <- s.isPaused
			
			// Only request world state after pause is acknowledged
			if s.isPaused {
				// Give time for the pause to be processed
				time.Sleep(100 * time.Millisecond)
				
				worldStateChannel := make(chan WorldState)
				s.worldStateChannel <- worldStateChannel
				
				select {
				case worldState := <-worldStateChannel:
					res.Turns = worldState.CurrentTurn
					fmt.Printf("Is paused")
				case <-time.After(1 * time.Second):
					fmt.Printf("Timeout waiting for world state")
				}
			} else {
				res.Message = "Continuing"
				fmt.Printf("Is not paused")
			}
		case "kill":
			fmt.Println("Kill command received")
			fmt.Println("Shutting down server...")
			// Close the listener to stop accepting new connections
			if listener != nil {
				listener.Close()
			}

			// Signal any running games to stop
			s.stopChannel <- true
			// Give a small delay for cleanup
			//time.Sleep(100 * time.Millisecond)
			defer os.Exit(0)
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
	var err error
    listener, err = net.Listen("tcp", "localhost:8030")
    if err != nil {
        fmt.Printf("Error starting server: %v\n", err)
        return
    }
	defer listener.Close()
	fmt.Printf("Server is listening on port %s...\n", *pAddr)
	rpc.Accept(listener)
}
