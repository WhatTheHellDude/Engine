package main

import (
	"context"
	"flag"
	"math/rand"
	"sync"
	"time"

	"github.com/battlesnakeio/engine/api"
	"github.com/battlesnakeio/engine/controller"
	"github.com/battlesnakeio/engine/controller/filestore"
	"github.com/battlesnakeio/engine/controller/pb"
	"github.com/battlesnakeio/engine/worker"
	log "github.com/sirupsen/logrus"
)

func init() { rand.Seed(time.Now().Unix()) }

func createStore(storeName string, saveDir string) controller.Store {
	stores := map[string]func() controller.Store{
		"memory": controller.InMemStore,
		"file": func() controller.Store {
			return filestore.NewFileStore(saveDir)
		},
	}

	newStore, ok := stores[storeName]
	if !ok {
		log.WithField("store", storeName).Fatal("Unknown storage option")
	}

	return newStore()
}

func main() {
	var (
		controllerAddr string
		apiAddr        string
		storeName      string
		saveDir        string
		workers        int
	)
	flag.StringVar(&controllerAddr, "controller listen", ":3004", "controller listen address.")
	flag.StringVar(&apiAddr, "api listen", ":3005", "api listen address")
	flag.StringVar(&storeName, "store", "memory", "game storage type (memory or file)")
	flag.StringVar(&saveDir, "save-dir", "~/.battlesnake/games", "location to store game files when using --store file")
	flag.IntVar(&workers, "workers", 10, "Worker count.")
	flag.Parse()

	c := controller.New(createStore(storeName, saveDir))
	go func() {
		log.Infof("controller listening on %s", controllerAddr)
		if err := c.Serve(controllerAddr); err != nil {
			log.Fatalf("controller failed to serve on (%s): %v", controllerAddr, err)
		}
	}()

	client, err := pb.Dial(controllerAddr)
	if err != nil {
		log.Fatalf("controller failed to dial (%s): %v", controllerAddr, err)
	}

	go func() {
		api := api.New(apiAddr, client)
		api.WaitForExit()
	}()

	w := &worker.Worker{
		ControllerClient: client,
		PollInterval:     1 * time.Second,
		RunGame:          worker.Runner,
	}

	ctx := context.Background()
	wg := &sync.WaitGroup{}
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			w.Run(ctx, i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
