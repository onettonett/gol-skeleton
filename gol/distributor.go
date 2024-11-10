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

type workerData struct {
    startY, endY int
    world        [][]uint8
    newWorld     [][]uint8
    params       Params
	turn		 int
	c 			 DistributorChannels
}


// distributor handles the main game logic and communication between components
func distributor(p Params, c DistributorChannels) {
	isPaused := false

	H := p.ImageHeight
	W := p.ImageWidth

	// Initialize turn counter and create the world grid
	turn := 0
	world := make([][]uint8, H)
	for i := 0; i < H; i++ {
		world[i] = make([]uint8, W)
	}
	
	// Request input from IO
	c.ioCommand <- ioInput

	filename := fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)
	c.ioFilename <- filename

	// Read the initial board state from IO
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			val := <-c.IoInput
			if(val==255){
				c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}
			}
			world[y][x] = val
		}
	}
	
	// Wait for IO to finish
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Executing}

	// Set up periodic reporting of alive cells
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)

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

	// Main game loop
	for i := 0; i < p.Turns; i++ {
		nextWorld := nextState(world, p, c, turn)
		
		world = nextWorld

		c.events <- TurnComplete{CompletedTurns: turn}
		turn++

		// Handle keyboard input
		select {
		case key := <-c.keyPresses:
			switch key {
				case 's': // Save current state
					saveBoardState(world, c, p, turn)
				case 'q': // Quit the game
					terminate(world, c, p, turn)
					return
				case 'p': // Pause/unpause the game
					isPaused = !isPaused
					if isPaused {
						c.events <- StateChange{turn, Paused}
						// Handle input while paused
						for isPaused {
							key := <-c.keyPresses
							switch key {
							case 'p': // Unpause
								isPaused = false
								c.events <- StateChange{turn, Executing}
							case 's': // Save while paused
								saveBoardState(world, c, p, turn)
							case 'q': // Quit while paused
								terminate(world, c, p, turn)
								return
							}
						}
					} else {
						c.events <- StateChange{turn, Executing}
					}
			}
		
		default:
		}

	}

	// Signal the reporting goroutine to stop
	done <- true

	// Report the final state using FinalTurnCompleteEvent
	alives := calculateAliveCells(world)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: alives}
	
	// Output the final state as a PGM image
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
	// Notify that the final turn is complete and send number of alive cells
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(world)}

	// Output the final board state to a PGM file
	saveBoardState(world, c, p, turn)

	// Update the game state to quitting
	c.events <- StateChange{turn, Quitting}
	
	// Clean up by closing events channel to prevent deadlock
	close(c.events)
}

func saveBoardState(world [][]uint8, c DistributorChannels, p Params, turn int) {
	// Send command to start PGM output
	c.ioCommand <- ioOutput
	// Generate filename based on dimensions and current turn
	filename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, turn)
	c.ioFilename <- filename
	
	// Send the current world state cell by cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	
	// Wait for IO operations to complete by checking idle status
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	
	// Notify that the image output is complete with turn number and filename
	c.events <- ImageOutputComplete{turn, filename}
}

// A worker processes a section of the Game of Life world grid in parallel
// It applies the rules of Conway's Game of Life:
// - Any live cell with fewer than 2 live neighbours dies (underpopulation)
// - Any live cell with 2 or 3 live neighbours lives
// - Any live cell with more than 3 live neighbours dies (overpopulation) 
// - Any dead cell with exactly 3 live neighbours becomes alive (reproduction)
// The function takes a workerData struct containing the section bounds and world state
// and signals completion through the done channel
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

// nextState calculates the next state of the Game of Life board by dividing work among multiple goroutines.
// It takes the current world state, game parameters, communication channels, and current turn number.
// Returns the new world state after applying the rules of Game of Life in parallel.
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

// countAliveNeighbours determines the number of alive neighbors for a cell at position (x,y).
// It implements periodic boundary conditions by wrapping around the edges of the world.
// Returns an integer count of alive neighbors (cells with value 255).
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

// calculateAliveCells scans the entire world grid and creates a list of all living cells.
// A cell is considered alive if its value is 255.
// Returns a slice of Cell structs containing the coordinates of all living cells.
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
