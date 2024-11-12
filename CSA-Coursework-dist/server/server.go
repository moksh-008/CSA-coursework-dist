package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var mutex sync.Mutex

// Helper function to calculate the next state of the world.
func calculateNextState(imageHeight, imageWidth int, world [][]uint8) [][]uint8 {
	newWorld := make([][]uint8, imageHeight)
	for i := range newWorld {
		newWorld[i] = make([]uint8, imageWidth)
	}

	countAliveNeighbors := func(x, y int) int {
		liveNeighbors := 0
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := (x+dx+imageWidth)%imageWidth, (y+dy+imageHeight)%imageHeight
				if world[ny][nx] == 255 {
					liveNeighbors++
				}
			}
		}
		return liveNeighbors
	}

	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			aliveNeighbors := countAliveNeighbors(x, y)
			currentCell := world[y][x]
			if currentCell == 255 && (aliveNeighbors == 2 || aliveNeighbors == 3) {
				newWorld[y][x] = 255
			} else if currentCell == 0 && aliveNeighbors == 3 {
				newWorld[y][x] = 255
			} else {
				newWorld[y][x] = 0
			}
		}
	}
	return newWorld
}

// Helper function to get the list of alive cells.
func getAliveCells(world [][]uint8) []util.Cell {
	var aliveCells []util.Cell
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world[y]); x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

type GameOfLifeServerOperations struct {
	world       [][]uint8
	imageHeight int
	imageWidth  int
	mutex       sync.Mutex
}

// GOL performs the Game of Life calculations over the specified number of turns.
func (s *GameOfLifeServerOperations) GOL(req stubs.Request, res *stubs.Response) (err error) {
	imageHeight := req.ImageHeight
	imageWidth := req.ImageWidth
	turns := req.Turns
	world := req.World

	// Initialize InterimAliveCounts with the number of turns
	res.InterimAliveCounts = make([]stubs.AliveCount, turns)

	// Process each turn and update the response
	for turn := 0; turn < turns; turn++ {
		world = calculateNextState(imageHeight, imageWidth, world)

		// Store alive cell count for the current turn
		aliveCount := len(getAliveCells(world))
		res.InterimAliveCounts[turn] = stubs.AliveCount{
			Turn:       turn + 1,
			AliveCells: aliveCount,
		}

		fmt.Printf("Turn %d: %d alive cells\n", turn+1, aliveCount)
	}

	// Set the final state in response
	res.FinalStateWorld = world
	res.AliveCells = getAliveCells(world)
	return
}

// reportAliveCells periodically reports the number of alive cells.
func (g *GameOfLifeServerOperations) AliveCells(req stubs.Request, res *stubs.Response) (err error) {
	mutex.Lock()
	aliveCells := getAliveCells(req.World)
	res.AliveCells = aliveCells
	res.CompletedTurns = req.Turns
	mutex.Unlock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// Register the Game of Life server operations with RPC
	server := &GameOfLifeServerOperations{}
	err := rpc.Register(server)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {

		}
	}(listener)
	fmt.Println("Game of Life server started on port", *pAddr)

	rpc.Accept(listener)
}
