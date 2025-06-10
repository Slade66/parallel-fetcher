package observer

import (
	"fmt"
	"strings"
	"sync"
)

// Observer 观察者接口
type Observer interface {
	// Update 是观察者接收通知的方法
	Update(downloaded int64)
}

// Observable 被观察者（主题）接口
type Observable interface {
	AddObserver(o Observer)
	Notify(downloaded int64)
}

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
		barWidth: 50, // 进度条在终端的显示宽度
	}
}

// Update 实现了 Observer 接口
func (p *ProgressBarObserver) Update(downloaded int64) {
	p.mu.Lock()
	p.current += downloaded
	p.mu.Unlock()

	p.print()
}

// print 在终端上绘制进度条
func (p *ProgressBarObserver) print() {
	p.mu.Lock()
	defer p.mu.Unlock()

	percent := float64(p.current) / float64(p.total)
	filledWidth := int(percent * float64(p.barWidth))

	// 构建进度条的视觉表示
	bar := strings.Repeat("=", filledWidth) + strings.Repeat(" ", p.barWidth-filledWidth)

	// 使用 \r 回到行首来刷新进度条，而不是每次都换行
	fmt.Printf("\r[%s] %.2f%% (%.2f/%.2f MB)",
		bar,
		percent*100,
		float64(p.current)/1024/1024,
		float64(p.total)/1024/1024,
	)

	// 如果下载完成，打印一个换行符
	if p.current >= p.total {
		fmt.Println()
	}
}
