package tx_test

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"github.com/go-naivecoin/tx"
	"encoding/hex"
	"github.com/decred/dcrd/dcrec/secp256k1"
)

const (
	ADDRESS = "04115c42e757b2efb7671c578530ec191a1359381e6a71127a9d37c486fd30dae57e76dc58f693bd7e7010358ce6b165e483a2921010db67ac11b1b51b651953d2"
	//TX_ID   = ""
)

func TestGetCoinbaseTransaction(t *testing.T) {

	transation := tx.GetCoinbaseTransaction(ADDRESS, 1)

	assert.Equal(t, "18105ee60d728d4ad229a15f5e76c396992f2d7ab4ddceaac77145d1843c17df", transation.Id)
}

func TestVerifyLogic(t *testing.T)  {
	pubKeyBytes, err := hex.DecodeString("04de21e72549de5f7b78fb04ac0fdca0e3c405f3fb970e4650f47808f9b0efe71dfa9fa5d80c0615873aad9b96e17b1220c07cfb1beffcb0b39721f583ffcab89d")

	assert.Nil(t, err)

	pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
	assert.Nil(t, err)

	sigBytes, err := hex.DecodeString("3045022100e0b193defe925732ea9652a5499eeb537faf464947596cddf6d4cf0ec989e78302204b6e455561b54d91558422f2198459df4cf324bf38e62e09a5b6ecd696607904")
	assert.Nil(t, err)

	signature, err := secp256k1.ParseDERSignature(sigBytes, pubKey.Curve)
	assert.Nil(t, err)

	res := signature.Verify([]byte("180ce43a30b8071b7548858ea419524c3bc6493e3d540b91b9f8e748e9c31614"), pubKey)
	assert.True(t, res)
}
