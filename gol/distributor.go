package gol

// acts as the client
import (
	"fmt"
	"uk.ac.bris.cs/gameoflife/util"
	"uk.ac.bris.cs/gameoflife/stubs"
	"net/rpc"
	"sync"
	"time"
)

type DistributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	IoInput    <-chan uint8
	keyPresses <- chan rune
}


var (
    rpcClient *rpc.Client
    clientMu  sync.Mutex
    once      sync.Once
)

// Add this function to manage the singleton connection
func getRPCClient() (*rpc.Client, error) {
    var err error
    once.Do(func() {
        rpcClient, err = rpc.Dial("tcp", "localhost:8030")
    })
    
    if err != nil {
        return nil, err
    }
    return rpcClient, nil
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

	// Executing each turn should be in the server
	// Client sends the world as an RPC call to the server
	// Server sends back the world after the turns have been done
	// Execute all turns of the Game of Life
	/*
	for i := 0; i < p.Turns; i++ {
		world = nextState(world, p, c)
		turn++
		c.events <- TurnComplete{CompletedTurns: turn}
	}
	*/

	// Initialise Ticker
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
				alives, turns := calculateAliveCellsNode()
				c.events <- AliveCellsCount{CompletedTurns: turns , CellsCount: alives}
			case <-done:
				return
			}
		}
	}()

	quit := make(chan bool)
    
    // Start goroutine to handle keypresses
    go func() {
        for {
            select {
            case key := <-c.keyPresses:
                switch key {
                case 's':
                    // Get current state from server
					saveBoardState(c, p)
                case 'q':
                    // Send quit command to server
                    client, err := getRPCClient()
                    if err == nil {
                        request := stubs.StateRequest{Command: "quit"}
                        response := new(stubs.StateResponse)
                        client.Call("SecretStringOperations.HandleCommand", request, response)
                    }
                case 'p':
                    // Send pause/resume command to server
                    client, err := getRPCClient()
                    if err == nil {
                        request := stubs.StateRequest{Command: "pause"}
                        response := new(stubs.StateResponse)
                        client.Call("SecretStringOperations.HandleCommand", request, response)
                    }
                }
            case <-quit:
                return
            }
        }
    }()


	world = nextStateNode(world, p)

	done <- true


	// TODO: Report the final state using FinalTurnCompleteEvent.
	alives := calculateAliveCells(world)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: alives}
	// send an event down an events channel
	// must implement the events channel, FinalTurnComplete is an event so must implement the event interface
	// Make sure that the Io has finished any output before exiting.

	//output the state of the board after all turns have been completed as a PGM image
	c.ioCommand <- ioOutput
	filename = fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)
	c.ioFilename <- filename
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// if it's idle it'll return true so you can use it before reading input, for example
	// to ensure output has saved before reading
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

/*
func main() {
	// connect to RPC server and send a request
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	request := stubs.Request{World: world, P: p, C: c}
	response := new(stubs.Response)
	client.Call(stubs.ReverseHandler, request, response)
	fmt.Println("Responded: ")
}
*/

func saveBoardState(c DistributorChannels, p Params) {
	client, err := getRPCClient()
	if err != nil {
		fmt.Printf("RPC failed: %v\n", err)
		return
	}
	request := stubs.StateRequest{Command: "save"}
	response := new(stubs.StateResponse)
	client.Call("SecretStringOperations.State", request, response)
	world := response.World
	turns := response.Turns


	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, turns)
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
	c.events <- ImageOutputComplete{turns, filename}
}

func calculateAliveCellsNode() (int, int) {
	client, err := getRPCClient()
	if err != nil {
		fmt.Printf("RPC failed: %v\n", err)
		return 0, 0
	}
	request := stubs.AliveCellsCountRequest{}
	response := new(stubs.AliveCellsCountResponse)
	client.Call("SecretStringOperations.AliveCellsCount", request, response)
	return response.CellsAlive, response.Turns
}

func nextStateNode(world [][]uint8, p Params) [][]uint8 {
	// Connect to RPC server
	client, err := getRPCClient()//rpc.Dial("tcp", "localhost:8030")
	if err != nil {
		// If we can't connect, fall back to local processing
		return nil
	}
	defer client.Close()

	// Create request with current world state and parameters
	request := stubs.Request{
		World: world,
		Turns: p.Turns,
		Threads: p.Threads,
		ImageWidth: p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	response := new(stubs.Response)

	// Make RPC call and get response
	err = client.Call("SecretStringOperations.Start", request, response)
	if err != nil {
		// If RPC fails, fall back to local processing
		fmt.Printf("RPC failed: %v\n", err)
		fmt.Printf("Error stack trace:\n%+v\n", err)
		return nil
	}

	return response.UpdatedWorld
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