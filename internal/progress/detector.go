// Package progress 提供多维度进展检测
package progress

import (
	"errors"
	"fmt"
	"strings"
)

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

// DetectorError 记录单个检测器在 Detect 过程中的失败
type DetectorError struct {
	Detector string
	Err      error
}

func (e *DetectorError) Error() string {
	return fmt.Sprintf("检测器 %s 失败: %v", e.Detector, e.Err)
}

func (e *DetectorError) Unwrap() error { return e.Err }

// Result 检测结果
type Result struct {
	HasProgress bool
	Source      string // 哪个检测器发现了进展
	Details     string
	// Failures 记录本次 Detect 中失败的检测器。
	// 部分失败不再被静默读作“无进展”，调用方可以据此告警或记录日志。
	Failures []DetectorError
}

// MultiDetector 组合多个检测器
type MultiDetector struct {
	detectors []Detector
}

// NewMultiDetector 创建多检测器
func NewMultiDetector(detectors ...Detector) *MultiDetector {
	return &MultiDetector{detectors: detectors}
}

// Detect 运行所有检测器，任一检测到进展即返回 true。
// 单个检测器失败不影响整体判断，但会记录在 Result.Failures 中；
// 若所有检测器都失败，则返回聚合错误（fail closed），绝不把整体失败当作“无进展”。
func (m *MultiDetector) Detect() (*Result, error) {
	var failures []DetectorError
	for _, d := range m.detectors {
		hasProgress, err := d.Detect()
		if err != nil {
			failures = append(failures, DetectorError{Detector: d.Name(), Err: err})
			continue
		}
		if hasProgress {
			return &Result{
				HasProgress: true,
				Source:      d.Name(),
				Details:     d.Details(),
				Failures:    failures,
			}, nil
		}
	}

	// 所有检测器均失败：聚合错误返回，让调用方可见
	if len(m.detectors) > 0 && len(failures) == len(m.detectors) {
		errs := make([]error, len(failures))
		for i := range failures {
			errs[i] = &failures[i]
		}
		return &Result{
			HasProgress: false,
			Source:      "none",
			Details:     "所有检测器均失败",
			Failures:    failures,
		}, errors.Join(errs...)
	}

	details := "所有检测器均未检测到进展"
	if len(failures) > 0 {
		names := make([]string, len(failures))
		for i := range failures {
			names[i] = failures[i].Detector
		}
		details = fmt.Sprintf("%s（部分检测器失败: %s）", details, strings.Join(names, ", "))
	}

	return &Result{
		HasProgress: false,
		Source:      "none",
		Details:     details,
		Failures:    failures,
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
