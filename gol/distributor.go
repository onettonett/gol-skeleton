package gol

import (
	"fmt"
	"time"
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
	world := make([][]uint8, H) // create a slice with rows equal to ImageHeight
	for i := 0; i < H; i++ {
		world[i] = make([]uint8, W) // initialise each row with columns equal to ImageWidth
	}
	c.ioCommand <- ioInput

	filename := fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)
	c.ioFilename <- filename

	// fill in the 2D slice
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			world[y][x] = <-c.IoInput
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Executing}

	// Ticker to report alive cell counts every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Channel to signal when all turns are complete
	done := make(chan bool)

	// Goroutine to report alive cells count every 2 seconds
	go func() {
		for {
			select {
			case <-ticker.C:
				alives := calculateAliveCells(world)
				c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: len(alives)}
			case <-done:
				return
			}
		}
	}()

	// Execute all turns of the Game of Life
	for i := 0; i < p.Turns; i++ {
		world = nextState(world, p, c)
		turn++
		c.events <- TurnComplete{CompletedTurns: turn}
	}

	// Signal the reporting goroutine to stop
	done <- true

	// Report the final state using FinalTurnCompleteEvent
	alives := calculateAliveCells(world)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: alives}

	// Ensure IO has finished any output before exiting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully to avoid deadlock.
	close(c.events)
}

// nextState calculates the next state of the board
func nextState(world [][]uint8, p Params, c DistributorChannels) [][]uint8 {

	H := p.ImageHeight
	W := p.ImageWidth

	// Create the new world state
	toReturn := make([][]uint8, H) // create a slice with rows equal to ImageHeight
	for i := 0; i < H; i++ {
		toReturn[i] = make([]uint8, W) // initialise each row with columns equal to ImageWidth
	}

	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			sum := countAliveNeighbours(world, x, y, W, H)

			if world[y][x] == 255 {
				// The cell was previously alive
				if sum < 2 || sum > 3 {
					toReturn[y][x] = 0
				} else if sum == 2 || sum == 3 {
					// Keep the cell alive
					toReturn[y][x] = 255
				}
			} else if world[y][x] == 0 {
				// The cell was previously dead
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

// countAliveNeighbours calculates the number of alive neighbours for a given cell
func countAliveNeighbours(world [][]uint8, x, y, width, height int) int {
	sum := 0
	directions := []struct{ dx, dy int }{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1},           {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	for _, d := range directions {
		nx, ny := (x+d.dx+width)%width, (y+d.dy+height)%height
		if world[ny][nx] == 255 {
			sum++
		}
	}

	return sum
}

// calculateAliveCells returns a list of coordinates for cells that are alive
func calculateAliveCells(world [][]uint8) []util.Cell {
	alives := make([]util.Cell, 0)
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world[y]); x++ {
			if world[y][x] == 255 {
				alives = append(alives, util.Cell{X: x, Y: y})
			}
		}
	}
	return alives
}
