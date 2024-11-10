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
	keyPresses <-chan rune
}

// workerData represents a section of the world for a worker to process
type workerData struct {
    startY, endY int
    world        [][]uint8
    newWorld     [][]uint8
    params       Params
	turn		 int
	c 			 DistributorChannels
}

// distributor constructs a filename based on parameters
// distributor sends the filename to the IO goroutine, which sends back an image byte-by-byte
// the distributor evolves the gol by an amount dictated by parameter
// finally, it sends the alive cells down this final turn complete event that's used by the testing suite

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c DistributorChannels) {
	isPaused := false

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
			val := <-c.IoInput
			if(val==255){
				c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}
			}
			world[y][x] = val
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
		nextWorld := nextState(world, p, c, turn)
		
		world = nextWorld

		c.events <- TurnComplete{CompletedTurns: turn}
		turn++


		select {
		case key := <-c.keyPresses:
			switch key {
				case 's':
					saveBoardState(world, c, p, turn)
				case 'q':
					terminate(world, c, p, turn)
					return
				case 'p':
					isPaused = !isPaused
					if isPaused {
						c.events <- StateChange{turn, Paused}
						// Enter pause loop
						for isPaused {
							key := <-c.keyPresses
							switch key {
							case 'p':
								isPaused = false
								c.events <- StateChange{turn, Executing}
							case 's':
								saveBoardState(world, c, p, turn)
							case 'q':
								terminate(world, c, p, turn)
								return
							}
						}
					} else {
						c.events <- StateChange{turn, Executing}
					}
			}
		
		default:
			// Non-blocking select
	}

	}

	// Signal the reporting goroutine to stop
	done <- true

	// Report the final state using FinalTurnCompleteEvent
	alives := calculateAliveCells(world)
	//c.events <- TurnComplete{CompletedTurns: turn}
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: alives}
	
	//output the state of the board after all turns have been completed as a PGM image
	c.ioCommand <- ioOutput
	filename = fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)
	c.ioFilename <- filename
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Ensure IO has finished any output before exiting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully to avoid deadlock.
	close(c.events)
}

func terminate(world [][]uint8, c DistributorChannels, p Params, turn int){
	// Send a FinalTurnComplete event
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(world)}
	// Save the final state as a PGM image
	saveBoardState(world, c, p, turn)
	// Send a StateChange event
	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully to avoid deadlock.
	close(c.events)
}

func saveBoardState(world [][]uint8, c DistributorChannels, p Params, turn int) {
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, turn)
	c.ioFilename <- filename
	
	// Send the current world state
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	
	// Wait for IO to complete
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	
	// Send ImageOutputComplete event
	c.events <- ImageOutputComplete{turn, filename}
}

// worker processes its assigned section of the world
func worker(data workerData, done chan<- bool) {
    for y := data.startY; y < data.endY; y++ {
        for x := 0; x < data.params.ImageWidth; x++ {
            sum := countAliveNeighbours(data.world, x, y, data.params.ImageWidth, data.params.ImageHeight)
            
            if data.world[y][x] == 255 {
                if sum < 2 || sum > 3 {
                    data.newWorld[y][x] = 0
					data.c.events <- CellFlipped{data.turn, util.Cell{X: x, Y: y}}
                } else {
                    data.newWorld[y][x] = 255
                }
            } else {
                if sum == 3 {
                    data.newWorld[y][x] = 255
					data.c.events <- CellFlipped{data.turn, util.Cell{X: x, Y: y}}
                } else {
                    data.newWorld[y][x] = 0
                }
            }
        }
    }
    done <- true
}

// nextState calculates the next state of the board using worker threads
func nextState(world [][]uint8, p Params, c DistributorChannels, turn int) [][]uint8 {
    // Create the new world state
    newWorld := make([][]uint8, p.ImageHeight)
    for i := 0; i < p.ImageHeight; i++ {
        newWorld[i] = make([]uint8, p.ImageWidth)
    }

    // Calculate rows per worker
    rowsPerWorker := p.ImageHeight / p.Threads
    remainingRows := p.ImageHeight % p.Threads

    // Create channel to wait for workers
    done := make(chan bool)

    // Start workers
    currentRow := 0
    for i := 0; i < p.Threads; i++ {
        // Calculate this worker's row range
        startY := currentRow
        endY := startY + rowsPerWorker
        if i == p.Threads-1 {
            endY += remainingRows // Last worker takes any remaining rows
        }
        
        // Prepare worker data
        data := workerData{
            startY:   startY,
            endY:     endY,
            world:    world,
            newWorld: newWorld,
            params:   p,
			turn: turn,
			c: c,
        }

        // Start the worker
        go worker(data, done)
        
        currentRow = endY
    }

    // Wait for all workers to complete
    for i := 0; i < p.Threads; i++ {
        <-done
    }

    return newWorld
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
