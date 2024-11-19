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

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	file := strconv.Itoa(p.ImageWidth)
	file = file + "x" + file
	c.ioCommand <- ioInput
	c.ioFilename <- file

	for y := range world {
		for x := range world[y] {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// Connect to the Game of Life server over RPC.
	client, err := rpc.Dial("tcp", "34.228.70.171:8030")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer client.Close()

	request := stubs.Request{
		InitialWorld: world,
		ImageWidth:   p.ImageWidth,
		ImageHeight:  p.ImageHeight,
		Turns:        p.Turns,
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

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

	simulationDuration := time.Duration(p.Turns) * 1 * time.Second
	time.Sleep(simulationDuration)

	finalResponse := new(stubs.Response)
	err = client.Call(stubs.ServerHandler, request, finalResponse)
	if err != nil {
		fmt.Println("Error fetching final state:", err)
		return
	}

	c.events <- FinalTurnComplete{
		CompletedTurns: finalResponse.CompletedTurns,
		Alive:          finalResponse.AliveCellsAfterFinalState,
	}

	outputPGM(p, c, finalResponse.FinalWorld, finalResponse.CompletedTurns)

	done <- true

}

// outputPGM saves the final world state to a PGM file.
func outputPGM(p Params, c distributorChannels, world [][]byte, completedTurns int) {

	c.ioCommand <- ioOutput
	outputFilename := fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)
	c.ioFilename <- outputFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

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
