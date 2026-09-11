package agent_test

import (
	"time"

	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func createMockSignature(privKey string, pubKey string, clusterID, version int64, keys *transport.SyncKeysPayload) *transport.SyncSignature {
	checksum := keys.Checksum()
	msg := transport.BuildSignableMessage(transport.PayloadTypeSyncKeyUpdate, clusterID, version, checksum)
	sigBase64, _ := engine.SignPayload(privKey, msg)
	return &transport.SyncSignature{
		Type:      transport.PayloadTypeSyncKeyUpdate,
		Algorithm: "ed25519",
		Checksum:  checksum,
		SigBase64: sigBase64,
		PublicKey: pubKey,
	}
}

func signMockResponse(privKey, pubKey string, resp *transport.SyncResponse) {
	if resp.Timestamp == 0 {
		resp.Timestamp = time.Now().Unix()
	}
	checksum := resp.PayloadChecksum()
	msg := transport.BuildSignableMessage(transport.PayloadTypeSyncPayload, resp.NodeID, resp.Timestamp, checksum)
	sigBase64, _ := engine.SignPayload(privKey, msg)
	resp.Signature = &transport.SyncSignature{
		Type:      transport.PayloadTypeSyncPayload,
		Algorithm: "ed25519",
		Checksum:  checksum,
		SigBase64: sigBase64,
		PublicKey: pubKey,
	}
}
