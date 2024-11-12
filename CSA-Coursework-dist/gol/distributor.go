package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var serverAddress = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	ioKeypress <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	flag.Parse()

	// Initialize and load the initial world state
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	filename := fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)
	c.ioCommand <- ioInput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	// Initial debug statement for verification
	initialAliveCount := countAliveCells(world)
	fmt.Println("Initial Alive Cell Count:", initialAliveCount)

	// Return if Turns == 0
	if p.Turns == 0 {
		sendFinalOutput(world, p, c)
		return
	}

	client, err := rpc.Dial("tcp", *serverAddress)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer client.Close()

	// Prepare the RPC request to send for each turn
	request := stubs.Request{
		World:       world,
		ImageHeight: p.ImageHeight,
		ImageWidth:  p.ImageWidth,
		Turns:       1, // Single turn processing at a time
	}
	var response stubs.Response

	// Ticker for every 2 seconds reporting alive cell count
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	done := make(chan bool)

	var mutex sync.Mutex
	go func() {
		for {
			select {
			case <-ticker.C:
				response := new(stubs.Response)
				mutex.Lock()
				client.Call(stubs.AliveCellReport, request, response)
				c.events <- AliveCellsCount{response.CompletedTurns, len(response.AliveCells)}
				mutex.Unlock()
			case <-done:
				return

			}

		}
	}()
	//go func() {
	//	for {
	//		select {
	//		case key := <-c.ioKeypress:
	//			switch key {
	//			case 's':
	//				response := new(stubs.Response)
	//				err := client.Call(stubs.KeyPressHandler, stubs.KeyPress{Key: 's'}, response)
	//				if err == nil {
	//					savePgmFile(c, world, p.ImageWidth, p.ImageHeight, response.CompletedTurns)
	//				}
	//			case 'q':
	//				response := new(stubs.Response)
	//				err := client.Call(stubs.KeyPressHandler, stubs.KeyPress{Key: 'q'}, response)
	//				if err == nil {
	//					sendFinalOutput(world, p, c)
	//					c.events <- StateChange{CompletedTurns: response.CompletedTurns, NewState: Quitting}
	//				}
	//				return
	//			case 'k':
	//				response := new(stubs.Response)
	//				err := client.Call(stubs.KeyPressHandler, stubs.KeyPress{Key: 'k'}, response)
	//				if err == nil {
	//					savePgmFile(c, world, p.ImageWidth, p.ImageHeight, response.CompletedTurns)
	//				}
	//				client.Call(stubs.ShutDownHandler, stubs.Request{}, new(stubs.Response))
	//				return
	//			case 'p':
	//				response := new(stubs.Response)
	//				err := client.Call(stubs.KeyPressHandler, stubs.KeyPress{Key: 'p'}, response)
	//				if err == nil {
	//					if response.NewState == stubs.Paused {
	//						fmt.Println("Paused at turn:", response.CompletedTurns)
	//					} else {
	//						fmt.Println("Continuing")
	//					}
	//				}
	//			}
	//		}
	//	}
	//}()

	// Process each turn individually

	request.World = world
	err = client.Call(stubs.ServerHandler, request, &response)
	if err != nil {
		fmt.Println("Error in RPC call:", err)
		done <- true
		return
	}
	// Process interim alive cell counts
	for _, aliveCount := range response.InterimAliveCounts {
		c.events <- AliveCellsCount{CompletedTurns: aliveCount.Turn, CellsCount: aliveCount.AliveCells}
	}
	// Update the world state after each turn
	world = response.FinalStateWorld
	finalAliveCount := countAliveCells(world)
	c.events <- AliveCellsCount{CompletedTurns: p.Turns, CellsCount: finalAliveCount}

	// Send the final output stateboard as a PGM file
	sendFinalOutput(world, p, c)
	done <- true
}

func savePgmFile(c distributorChannels, world [][]byte, imageWidth, imageHeight, turn int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%dx%dx%d", imageWidth, imageHeight, turn) // taken from test go file
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			c.ioOutput <- world[y][x] // send pixel data to output channel
		}
	}

	// Make sure to wait for I/O to finish before signaling completion
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{turn, fmt.Sprintf("%dx%dx%d", imageWidth, imageHeight, turn)}
}

// Helper function to send the final state and events after all turns are complete
func sendFinalOutput(world [][]byte, p Params, c distributorChannels) {
	// Output the final state to IO channels
	c.ioCommand <- ioOutput
	outputFilename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)
	c.ioFilename <- outputFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Send final events after completion
	aliveCells := getAliveCells(world)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: aliveCells}
	c.events <- StateChange{CompletedTurns: p.Turns, NewState: Quitting}

	// Ensure IO has completed any pending tasks before quitting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	close(c.events)
}

// Helper function to count alive cells in the world
func countAliveCells(world [][]byte) int {
	aliveCount := 0
	for _, row := range world {
		for _, cell := range row {
			if cell == 255 {
				aliveCount++
			}
		}
	}
	return aliveCount
}

// Helper function to get a list of alive cell coordinates in the world
func getAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for y, row := range world {
		for x, cell := range row {
			if cell == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}
