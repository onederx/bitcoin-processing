package main

import (
	"log"
	"sync"
)

type runnable interface {
	Run() error
}

type processingComponentRunner struct {
	wg     *sync.WaitGroup
	failed bool
}

func newProcessingComponentRunner() *processingComponentRunner {
	return &processingComponentRunner{wg: &sync.WaitGroup{}}
}

func (r *processingComponentRunner) run(component runnable, name string, doneCh chan<- struct{}) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer close(doneCh)

		err := component.Run()
		if err != nil {
			r.failed = true
			log.Printf("%s has stopped with error %v", name, err)
		} else {
			log.Printf("%s has stopped normally", name)
		}
	}()
}

func (r *processingComponentRunner) wait() {
	r.wg.Wait()
}
