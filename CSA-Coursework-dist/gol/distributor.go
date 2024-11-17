package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

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
	// Initialize a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	file := strconv.Itoa(p.ImageWidth)
	file = file + "x" + file
	c.ioCommand <- ioInput
	c.ioFilename <- file

	// Populate the world with data read from the input PGM file.
	for y := range world {
		for x := range world[y] {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// Connect to the Game of Life server over RPC.
	client, err := rpc.Dial("tcp", ":8030")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer client.Close()

	// Prepare a request to send to the server with the initial world state and parameters.
	request := stubs.Request{
		InitialWorld: world,
		ImageWidth:   p.ImageWidth,
		ImageHeight:  p.ImageHeight,
		Turns:        p.Turns,
	}

	// Set up a ticker to call the `Alive` method every 2 seconds.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Channel to signal when the simulation is complete.
	done := make(chan bool)

	// Start a goroutine for periodic alive cell count requests.
	go func() {

		for {
			select {
			case <-ticker.C:

				aliveResponse := new(stubs.AliveResponse)
				aliveRequest := stubs.AliveRequest{ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth}
				err := client.Call(stubs.AliveCellReport, aliveRequest, aliveResponse)
				if err != nil {
					fmt.Println("Error in Alive RPC call:", err)
					continue
				}
				// Emit an AliveCellsCount event with the current alive cell count.
				c.events <- AliveCellsCount{
					CompletedTurns: aliveResponse.Turn,
					CellsCount:     aliveResponse.AliveCellsCount,
				}
			case <-done:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case command := <-c.ioKeypress:
				keyRequest := stubs.KeyRequest{command}
				keyResponse := new(stubs.KeyResponse)
				err := client.Call(stubs.KeyPresshandler, keyRequest, keyResponse)
				if err != nil {
					log.Fatal("Key Press Call Error:", err)
				}
				outFileName := file + "x" + strconv.Itoa(keyResponse.Turns)
				switch command {
				case 's':
					c.events <- StateChange{keyResponse.Turns, Executing}
					savePGMImage(c, keyResponse.World, outFileName, p.ImageHeight, p.ImageWidth)
				case 'k':
					err := client.Call(stubs.KillServerHandler, stubs.KillRequest{}, new(stubs.KillResponse))
					savePGMImage(c, keyResponse.World, outFileName, p.ImageHeight, p.ImageWidth)
					c.events <- StateChange{keyResponse.Turns, Quitting}
					if err != nil {
						log.Fatal("Kill Request Call Error:", err)
					}
					done <- true
				case 'q':
					c.events <- StateChange{keyResponse.Turns, Quitting}
					done <- true
				case 'p':
					paused := true
					fmt.Println(keyResponse.Turns)
					c.events <- StateChange{keyResponse.Turns, Paused}
					for paused == true {
						command := <-c.ioKeypress
						switch command {
						case 'p':
							keyRequest := stubs.KeyRequest{command}
							keyResponse := new(stubs.KeyResponse)
							client.Call(stubs.KeyPresshandler, keyRequest, keyResponse)
							c.events <- StateChange{keyResponse.Turns, Executing}
							fmt.Println("Continuing")
							paused = false
						}
					}
				}
			}
		}
	}()
	// Make the RPC call to the server's Game of Life handler to start the simulation.
	err = client.Call(stubs.ServerHandler, request, &stubs.Response{})
	if err != nil {
		fmt.Println("Error in GOL RPC call:", err)
		return
	}

	// Wait until the simulation completes.
	// Since the GOL RPC call returns immediately, we need another way to determine completion.
	// For simplicity, we'll wait for the number of turns multiplied by the delay per turn.
	// Alternatively, implement a mechanism to detect simulation completion via additional RPC or shared state.
	simulationDuration := time.Duration(p.Turns) * 1 * time.Second
	time.Sleep(simulationDuration)

	// After simulation completes, request the final world state.
	finalResponse := new(stubs.Response)
	err = client.Call(stubs.ServerHandler, request, finalResponse)
	if err != nil {
		fmt.Println("Error fetching final state:", err)
		return
	}

	// Send the final world state and list of alive cells to the events channel.
	c.events <- FinalTurnComplete{
		CompletedTurns: finalResponse.CompletedTurns,
		Alive:          finalResponse.AliveCellsAfterFinalState,
	}

	// Output the final world state to a PGM file.
	outputPGM(p, c, finalResponse.FinalWorld, finalResponse.CompletedTurns)

	// Signal that the simulation is complete.
	done <- true

}

// outputPGM saves the final world state to a PGM file.
func outputPGM(p Params, c distributorChannels, world [][]byte, completedTurns int) {
	// Output the final state to IO channels
	c.ioCommand <- ioOutput
	outputFilename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)
	c.ioFilename <- outputFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Ensure IO has completed any pending tasks before quitting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{completedTurns, Quitting}
	close(c.events)
}
func savePGMImage(c distributorChannels, w [][]byte, file string, imageHeight, imageWidth int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- file
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			c.ioOutput <- w[y][x]
		}
	}
}
