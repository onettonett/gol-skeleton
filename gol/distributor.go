package gol

import (
	"fmt"
	"uk.ac.bris.cs/gameoflife/util"
)

type DistributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	IoInput    <-chan uint8
}

// distributor constructs a filename based on parameters
// distributor sends the filename to the IO goroutine, which sends back an image byte-by-byte
// the distributor evolves the gol by an amount dictated by parameter
// finally, it sends the alive cells down this final turn complete event that's used by the testing suite

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c DistributorChannels) {

	// first thing is to get the image

	// TODO: Create a 2D slice to store the world.
	H := p.ImageHeight
	W := p.ImageWidth

	turn := 0
	world := make([][]uint8, H) // create a slice with 16 rows
	for i := 0; i < H; i++ {
		world[i] = make([]uint8, W) // initialise each row with 16 columns
	}
	c.ioCommand <- ioInput

	filename := fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)
	c.ioFilename <- filename

	// fill in the 2d slice
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			world[y][x] = <-c.IoInput
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Executing}
	// TODO: Execute all turns of the Game of Life.
	for i := 0; i < p.Turns; i++ {
		world = nextState(world, p, c)
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	alives := calculateAliveCells(world)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: alives}
	// send an event down an events channel
	// must implement the events channel, FinalTurnComplete is an event so must implement the event interface
	// Make sure that the Io has finished any output before exiting.

	// if it's idle it'll return true so you can use it before reading input, for example
	// to ensure output has saved before reading
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func nextState(world [][]uint8, p Params, c DistributorChannels) [][]uint8 {

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

func calculateAliveCells(world [][]byte) []util.Cell {
	alives := make([]util.Cell, 0)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if world[y][x] == 255 {
				newCell := util.Cell{x, y}
				alives = append(alives, newCell)
			}
		}
	}
	return alives
}
