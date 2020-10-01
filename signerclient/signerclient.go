package signerclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

type SignerClient struct {
	BakerPkh     string
	BakerPk      string
	SignerURL    string
	client       *http.Client
}

type SignerResult struct {
	Signature string `json:"signature"`
}

// SignOperationOutput contains an operation with the signature appended, and the signature
type SignOperationOutput struct {
	SignedOperation string
	Signature       string
	EDSig           string
}

type PublicKeyResult struct {
	PublicKey string `json:"public_key"`
}

func New(bakerPkh, signerUrl string) (*SignerClient, error) {

	sc := &SignerClient{
		BakerPkh: bakerPkh,
		SignerURL: signerUrl,
		client: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout: 10 * time.Second,
				}).Dial,
				DisableCompression: true,
				DisableKeepAlives: true,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}

	// Check that signer works by fetching the public key for the baker
	publicKeyBytes, err := sc.getPublicKey()
	if err != nil {
		return nil, errors.Wrap(err, "Could not fetch public key")
	}

	var publicKey PublicKeyResult
	if err := json.Unmarshal(publicKeyBytes, &publicKey); err != nil {
		return nil, errors.Wrap(err, "Could not unmarshal public key")
	}
	
	sc.BakerPk = publicKey.PublicKey

	return sc, nil
}

func (s *SignerClient) SignEndorsement(endorsementBytes string, chainID string) (SignOperationOutput, error) {

	// Strip off the chainId prefix, and then base58 decode the chain id string (ie: NetXUdfLh6Gm88t)
	chainIdBytes := b58cdecode(chainID, chainidprefix)
	endorsementWatermark := append(endorsementprefix, chainIdBytes...)

	opBytes, err := hex.DecodeString(endorsementBytes)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed to sign operation")
	}
	opBytes = append(endorsementWatermark, opBytes...)
	finalOpHex := strconv.Quote(hex.EncodeToString(opBytes))  // Must be quote-wrapped

	respBytes, err := s.signOperation(finalOpHex)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed signer")
	}

	// Unmarshal response from signer
	var edSig SignerResult
	if err := json.Unmarshal(respBytes, &edSig); err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed to unmarshal signer")
	}

	// Decode out the signature from the operation
	decodedSig, err := decodeSignature(edSig.Signature)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed to decode signature")
	}

	return SignOperationOutput{
		SignedOperation: fmt.Sprintf("%s%s", endorsementBytes, decodedSig),
		Signature: decodedSig,
		EDSig: edSig.Signature,
	}, nil
}

func (s *SignerClient) signOperation(data string) ([]byte, error) {

	signerPath := fmt.Sprintf("%s/keys/%s", s.SignerURL, s.BakerPkh)

	req, err := http.NewRequest(http.MethodPost, signerPath, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct POST request")
	}

	return s.makeRequest(req)
}

func (s *SignerClient) getPublicKey() ([]byte, error) {

	signerPath := fmt.Sprintf("%s/keys/%s", s.SignerURL, s.BakerPkh)

	req, err := http.NewRequest(http.MethodGet, signerPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct GET request")
	}

	return s.makeRequest(req)
}

func (s *SignerClient) makeRequest(req *http.Request) ([]byte, error) {

	req.Header.Set("Content-Type", "application/json")

	// Execute POST
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}

	// Read response body
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read response body")
	}

	// Check HTTP response result
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response returned code %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	// Close connection
	s.client.CloseIdleConnections()

	return bodyBytes, nil
}