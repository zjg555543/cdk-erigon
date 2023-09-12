/*
   Copyright 2022 Erigon-Lightclient contributors
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

package sentinel

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// RandomPeer gets and claims a random peer for use
// caller MUST call the free func to free the peer after calling!!
func (s *Sentinel) RandomPeer(topic string) (pid peer.ID, free func(), err error) {
	pid, free, err = connectToRandomPeer(s, string(BeaconBlockTopic))
	if err != nil {
		return peer.ID(""), free, fmt.Errorf("failed to connect to a random peer err=%s", err)
	}
	return pid, free, nil
}
