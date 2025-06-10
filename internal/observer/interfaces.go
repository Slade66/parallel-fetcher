// internal/observer/interfaces.go
package observer

// Observer 观察者接口
type Observer interface {
	Update(downloaded int64)
}

// Observable 被观察者（主题）接口
type Observable interface {
	AddObserver(o Observer)
	Notify(downloaded int64)
}
