package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
)

// distributor.go acts as the client
// server file is on the server
// go run server/server.go
// pressed green button in distributor

// Secret method that we can't let clients see
func nextState(world [][]uint8, p gol.Params, c gol.DistributorChannels) [][]uint8 {

	H := p.ImageHeight
	W := p.ImageHeight

	// make toReturn 2d slice
	toReturn := make([][]uint8, H) // create a slice with 16 rows
	for i := 0; i < H; i++ {
		toReturn[i] = make([]uint8, W) // initialise each row with 16 columns
	}
	// fill in the 2d slice
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			toReturn[y][x] = <-c.IoInput
		}
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

func doAllTurns(world [][]uint8, p gol.Params, c gol.DistributorChannels) {
	for i := 0; i < p.Turns; i++ {
		world = nextState(world, p, c)
	}
}

type SecretStringOperations struct{}

// this is like the Reverse method in SecretStrings
func (s *SecretStringOperations) Update(req stubs.Request, res *stubs.Response) (err error) {
	res.UpdatedWorld = nextState(req.World, req.P, req.C)
	// func nextState(world [][]uint8, p gol.Params, c gol.DistributorChannels) [][]uint8
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&SecretStringOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
