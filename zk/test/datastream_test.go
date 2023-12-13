package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon/zk/datastream"
	"github.com/ledgerwatch/erigon/zk/datastream/client"
	"github.com/ledgerwatch/erigon/zk/datastream/types"
)

func generateTestCases(maxBlock uint64, blockIncrement uint64) map[string]struct {
	fromBlock uint64
	toBlock   uint64
} {
	testCases := make(map[string]struct {
		fromBlock uint64
		toBlock   uint64
	})

	maxBlock = maxBlock
	blockIncrement = blockIncrement

	for fromBlock := uint64(0); fromBlock < maxBlock; fromBlock += blockIncrement {
		toBlock := fromBlock + blockIncrement
		testName := fmt.Sprintf("test_block_%d_%d", fromBlock, toBlock)
		testCases[testName] = struct {
			fromBlock uint64
			toBlock   uint64
		}{
			fromBlock: fromBlock,
			toBlock:   toBlock,
		}
	}

	return testCases
}

func parseUintEnvVariable(t *testing.T, envVarName string) uint64 {
	envVarValue := os.Getenv(envVarName)
	value, err := strconv.ParseUint(envVarValue, 10, 64)
	if err != nil {
		t.Fatalf("Error parsing %s: %v", envVarName, err)
	}
	return value
}

func TestDatastream(t *testing.T) {

	streamUrl := os.Getenv("streamUrl")
	if streamUrl == "" {
		t.Fatal("streamUrl environment variable not provided")
	}

	maxBlock := parseUintEnvVariable(t, "maxBlock")
	blockIncrement := parseUintEnvVariable(t, "blockIncrement")

	var testCases = generateTestCases(maxBlock, blockIncrement)
	for test_name, tc := range testCases {
		tc := tc
		tn := test_name

		blockChan := make(chan types.FullL2Block)
		gerUpdateChan := make(chan types.GerUpdate)

		t.Run(test_name, func(t *testing.T) {
			t.Parallel()

			go func() {
				for {
					if _, _, err := datastream.DownloadAllL2BlocksToChannel(streamUrl, blockChan, gerUpdateChan,
						tc.fromBlock); err != nil {
						if errors.Is(err, client.ErrBadBookmark) {
							t.Fail()
							t.Logf("Bad Bookmark")
							break
						}
					}
				}
			}()

			receivedBlocks := make(map[uint64]types.FullL2Block)

			for {
				select {

				case l2Block, ok := <-blockChan:
					if !ok {
						t.Fail()
						t.Logf("L2 Block channel closed")
						break
					}
					if l2Block.L2BlockNumber >= tc.toBlock {
						fmt.Printf("%s received all blocks - checking!\n", tn)
						missingBlocks := checkContiguousBlocks(receivedBlocks, tc.toBlock, tc.fromBlock)
						if len(missingBlocks) != 0 {
							t.Fail()
							t.Logf("There are missing blocks: %d", missingBlocks)
						}
						return
					}
					receivedBlocks[l2Block.L2BlockNumber] = l2Block
					if l2Block.L2BlockNumber%10000 == 0 {
						fmt.Printf("%s received block %d\n", tn, l2Block.L2BlockNumber)
					}

				case gerUpdate, ok := <-gerUpdateChan:
					if !ok {
						log.Println("GER Update channel closed")
					}
					_ = gerUpdate
					// Handle GER   if necessary
				}
			}
		})
	}
}

func checkContiguousBlocks(receivedBlocks map[uint64]types.FullL2Block, toBlockNumber uint64, fromBlockNumber uint64) []uint64 {
	var expectedBlockNumber uint64 = fromBlockNumber
	var missingBlocks []uint64

	for {
		if expectedBlockNumber >= toBlockNumber {
			break
		}
		if _, ok := receivedBlocks[expectedBlockNumber]; !ok {
			missingBlocks = append(missingBlocks, expectedBlockNumber)

		}
		expectedBlockNumber++
	}
	return missingBlocks
}