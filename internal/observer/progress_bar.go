// internal/observer/progress_bar.go
package observer

import (
	"fmt"
	"strings"
	"sync"
)

// ProgressBarObserver 是一个具体的观察者，用于显示终端进度条
type ProgressBarObserver struct {
	total    int64
	current  int64
	barWidth int
	mu       sync.Mutex
}

// NewProgressBarObserver 创建一个新的进度条观察者
func NewProgressBarObserver(totalSize int64) *ProgressBarObserver {
	return &ProgressBarObserver{
		total:    totalSize,
		barWidth: 50,
	}
}

// Update 实现了 Observer 接口
func (p *ProgressBarObserver) Update(downloaded int64) {
	p.mu.Lock()
	p.current += downloaded
	p.mu.Unlock()
	p.print()
}

// print 在终端上绘制进度条 (代码不变)
func (p *ProgressBarObserver) print() {
	p.mu.Lock()
	defer p.mu.Unlock()
	percent := float64(p.current) / float64(p.total)
	filledWidth := int(percent * float64(p.barWidth))
	bar := strings.Repeat("=", filledWidth) + strings.Repeat(" ", p.barWidth-filledWidth)
	fmt.Printf("\r[%s] %.2f%% (%.2f/%.2f MB)",
		bar,
		percent*100,
		float64(p.current)/1024/1024,
		float64(p.total)/1024/1024,
	)
	if p.current >= p.total {
		fmt.Println()
	}
}
