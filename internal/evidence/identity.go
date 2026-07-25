package evidence

import (
	"crypto/sha256"
	"fmt"
)

func EvidenceIDForChunk(chunkID string) string {
	return fmt.Sprintf("ev_%x", sha256.Sum256([]byte(chunkID)))
}
