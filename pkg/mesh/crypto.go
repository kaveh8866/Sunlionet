package mesh

import (
	"crypto/rand"
	"encoding/json"
	"errors"

	"golang.org/x/crypto/nacl/box"
)

var (
	errDecrypt = errors.New("mesh decrypt failed")
	errDecode  = errors.New("mesh payload decode failed")
)

type MeshMessage struct {
	Nonce      [24]byte `json:"nonce"`
	Ciphertext []byte   `json:"ciphertext"`
	SenderPub  [32]byte `json:"sender_pub"`
}

type ProxyOffer struct {
	Timestamp int64  `json:"timestamp"`
	Config    string `json:"config_base64"`
	HopCount  int    `json:"hop_count"`
}

type Crypto struct {
	privKey [32]byte
	pubKey  [32]byte
}

func NewCrypto() (*Crypto, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Crypto{
		privKey: *priv,
		pubKey:  *pub,
	}, nil
}

func (c *Crypto) PublicKey() [32]byte {
	return c.pubKey
}

func (c *Crypto) EncryptPayload(plaintext []byte, recipientPub [32]byte) (MeshMessage, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return MeshMessage{}, err
	}
	rpub := recipientPub
	ciphertext := box.Seal(nil, plaintext, &nonce, &rpub, &c.privKey)
	return MeshMessage{
		Nonce:      nonce,
		Ciphertext: ciphertext,
		SenderPub:  c.pubKey,
	}, nil
}

func (c *Crypto) DecryptPayload(msg MeshMessage) ([]byte, error) {
	plaintext, ok := box.Open(nil, msg.Ciphertext, &msg.Nonce, &msg.SenderPub, &c.privKey)
	if !ok {
		return nil, errDecrypt
	}
	return plaintext, nil
}

func (c *Crypto) EncryptOffer(offer ProxyOffer, recipientPub [32]byte) (MeshMessage, error) {
	plaintext, err := json.Marshal(offer)
	if err != nil {
		return MeshMessage{}, err
	}
	return c.EncryptPayload(plaintext, recipientPub)
}

func (c *Crypto) DecryptOffer(msg MeshMessage) (ProxyOffer, error) {
	plaintext, err := c.DecryptPayload(msg)
	if err != nil {
		return ProxyOffer{}, err
	}

	var offer ProxyOffer
	if err := json.Unmarshal(plaintext, &offer); err != nil {
		return ProxyOffer{}, errDecode
	}

	return offer, nil
}

func (c *Crypto) ForwardMessage(msg MeshMessage, nextHopPub [32]byte) (MeshMessage, error) {
	offer, err := c.DecryptOffer(msg)
	if err != nil {
		return MeshMessage{}, err
	}
	offer.HopCount++
	return c.EncryptOffer(offer, nextHopPub)
}
