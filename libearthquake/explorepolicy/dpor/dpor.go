// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpor

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	. "../../equtils"
	. "../../historystorage"
)

type DPORParam struct {
	interval time.Duration // in millisecond
}

type DPOR struct {
	nextEventChan chan *Event
	randGen       *rand.Rand
	queueMutex    *sync.Mutex

	eventQueue []*Event // high priority

	param *DPORParam
}

func constrDPORParam(rawParam map[string]interface{}) *DPORParam {
	var param DPORParam

	if _, ok := rawParam["interval"]; ok {
		param.interval = time.Duration(int(rawParam["interval"].(float64)))
	} else {
		param.interval = time.Duration(100) // default: 100ms
	}

	return &param
}

// FIXME: unify functions in visualize.go

func createTracesPerEntity(trace []Event) map[string][]Event {
	perEntity := make(map[string][]Event)

	for _, ev := range trace {
		if _, ok := perEntity[ev.ProcId]; !ok {
			perEntity[ev.ProcId] = make([]Event, 0)
		}

		perEntity[ev.ProcId] = append(perEntity[ev.ProcId], ev)
	}

	return perEntity
}

func reducePartialOrder(trace []Event) []Event {
	perEntity := createTracesPerEntity(trace)

	keys := make([]string, len(perEntity))

	i := 0
	for k, _ := range perEntity {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	reduced := make([]Event, 0)
	for _, key := range keys {
		for _, ev := range perEntity[key] {
			reduced = append(reduced, ev)
		}
	}

	return reduced
}

func (this *DPOR) Init(storage HistoryStorage, param map[string]interface{}) {
	this.param = constrDPORParam(param)

	prefix := make([]Event, 0)

	go func() {
		for {
			time.Sleep(this.param.interval * time.Millisecond)

			this.queueMutex.Lock()
			if len(this.eventQueue) == 0 {
				Log("no event is queued")
				this.queueMutex.Unlock()
				continue
			}

			nextIdx := -1
			for i := 0; i < len(this.eventQueue); i++ {
				// TODO: match every event in the queue, wait for more intervals, etc
				tmpprefix := append(prefix, *this.eventQueue[i])

				matched := storage.SearchWithConverter(reducePartialOrder(tmpprefix), reducePartialOrder)
				if len(matched) == 0 {
					nextIdx = i
					break
				}
			}

			if nextIdx == -1 {
				nextIdx = this.randGen.Int() % len(this.eventQueue)
			}

			next := this.eventQueue[nextIdx]
			this.eventQueue = append(this.eventQueue[:nextIdx], this.eventQueue[nextIdx+1:]...)

			this.queueMutex.Unlock()

			prefix = append(prefix, *next)

			this.nextEventChan <- next
		}
	}()
}

func (this *DPOR) Name() string {
	return "DPOR"
}

func (this *DPOR) GetNextEventChan() chan *Event {
	return this.nextEventChan
}

func (this *DPOR) QueueNextEvent(procId string, ev *Event) {
	this.queueMutex.Lock()

	this.eventQueue = append(this.eventQueue, ev)

	this.queueMutex.Unlock()
}

func DPORNew() *DPOR {
	nextEventChan := make(chan *Event)
	eventQueue := make([]*Event, 0)
	mutex := new(sync.Mutex)
	r := rand.New(rand.NewSource(time.Now().Unix()))

	return &DPOR{
		nextEventChan,
		r,
		mutex,
		eventQueue,
		nil,
	}
}
