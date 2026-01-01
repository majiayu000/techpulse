package progress

import (
	"fmt"

	"github.com/anthropic/autonomous-runner/internal/memory"
)

// HashDetector 基于文件哈希检测进展
type HashDetector struct {
	memory   *memory.Manager
	lastHash string
	currHash string
	prevHash string // 用于 Details() 显示变化
}

// NewHashDetector 创建哈希检测器
func NewHashDetector(mem *memory.Manager) (*HashDetector, error) {
	hash, err := mem.ContentHash()
	if err != nil {
		return nil, err
	}
	return &HashDetector{
		memory:   mem,
		lastHash: hash,
	}, nil
}

func (h *HashDetector) Name() string {
	return "hash"
}

func (h *HashDetector) Detect() (bool, error) {
	hash, err := h.memory.ContentHash()
	if err != nil {
		return false, err
	}

	h.prevHash = h.lastHash // 保存旧值用于显示
	h.currHash = hash
	hasProgress := hash != h.lastHash
	h.lastHash = hash

	return hasProgress, nil
}

func (h *HashDetector) Reset() error {
	hash, err := h.memory.ContentHash()
	if err != nil {
		return err
	}
	h.lastHash = hash
	return nil
}

func (h *HashDetector) Details() string {
	return fmt.Sprintf("hash: %s -> %s", h.prevHash, h.currHash)
}
