package zkchainconfig

var chainIds = []uint64{
	195,
	1001,
	1101,
	1440,
	1442,
}

func IsZk(chainId uint64) bool {
	for _, validId := range chainIds {
		if chainId == validId {
			return true
		}
	}
	return false
}

func IsTestnet(chainId uint64) bool {
	return chainId == 1442
}

func IsDevnet(chainId uint64) bool {
	return chainId == 1440
}

func CheckForkOrder() error {
	return nil
}
