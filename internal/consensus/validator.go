package consensus

type Validator struct {
	PublicKey          []byte
	Stake              uint64
	LastBlockProduced  uint64
	IsActive           bool
}
