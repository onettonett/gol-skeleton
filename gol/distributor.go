package gol

// acts as the client
import (
	"flag"
	"fmt"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
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

	// Executing each turn should be in the server
	// Client sends the world as an RPC call to the server
	// Server sends back the world after the turns have been done
	// TODO: Execute all turns of the Game of Life.

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
