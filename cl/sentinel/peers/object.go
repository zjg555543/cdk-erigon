package peers

type PeeredObject[T any] struct {
	Peer string
	Data T
}
