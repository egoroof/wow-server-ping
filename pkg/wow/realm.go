package wow

type Realm struct {
	realmType  byte
	locked     byte
	flag       byte
	Name       string
	Address    string
	population []byte
	numChars   byte
	category   byte
	realmId    byte
}
