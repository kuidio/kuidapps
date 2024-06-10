/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package devbuilder

import "sync"

func newRRs() *rrs {
	return &rrs{
		rrs: map[string]*rr{},
	}
}

type rrs struct {
	m     sync.RWMutex
	rrs map[string]*rr
}

type rr struct {
	ip string
	ipv4 bool
}

func (r *rrs) List() map[string]*rr {
	r.m.RLock()
	defer r.m.RUnlock()

	rrs := make(map[string]*rr, len(r.rrs))
	for name, rr := range r.rrs {
		rrs[name] = rr
	}
	return rrs
}

func (r *rrs) Add(name string, rr *rr) {
	r.m.Lock()
	defer r.m.Unlock()

	r.rrs[name] = rr
}
