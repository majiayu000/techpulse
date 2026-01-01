// Package progress 提供多维度进展检测
package progress

// Detector 进展检测器接口
type Detector interface {
	// Name 返回检测器名称
	Name() string
	// Detect 检测是否有进展
	Detect() (bool, error)
	// Reset 重置检测器状态（用于开始新一轮检测）
	Reset() error
	// Details 返回检测详情（用于日志）
	Details() string
}

// Result 检测结果
type Result struct {
	HasProgress bool
	Source      string // 哪个检测器发现了进展
	Details     string
}

// MultiDetector 组合多个检测器
type MultiDetector struct {
	detectors []Detector
}

// NewMultiDetector 创建多检测器
func NewMultiDetector(detectors ...Detector) *MultiDetector {
	return &MultiDetector{detectors: detectors}
}

// Detect 运行所有检测器，任一检测到进展即返回 true
func (m *MultiDetector) Detect() (*Result, error) {
	for _, d := range m.detectors {
		hasProgress, err := d.Detect()
		if err != nil {
			// 单个检测器失败不影响整体
			continue
		}
		if hasProgress {
			return &Result{
				HasProgress: true,
				Source:      d.Name(),
				Details:     d.Details(),
			}, nil
		}
	}

	return &Result{
		HasProgress: false,
		Source:      "none",
		Details:     "所有检测器均未检测到进展",
	}, nil
}

// Reset 重置所有检测器
func (m *MultiDetector) Reset() error {
	for _, d := range m.detectors {
		if err := d.Reset(); err != nil {
			return err
		}
	}
	return nil
}
