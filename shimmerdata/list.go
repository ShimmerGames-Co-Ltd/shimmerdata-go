package shimmerdata

import (
	"container/list"
	"sync"
)

// SafeList 是一个并发安全的链表
type SafeList struct {
	list  *list.List
	mutex sync.Mutex
}

// NewSafeList 创建一个新的 SafeList
func NewSafeList() *SafeList {
	return &SafeList{
		list: list.New(),
	}
}

// PushBack 添加元素到链表尾部
func (sl *SafeList) PushBack(value interface{}) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	sl.list.PushBack(value)
}

// PushFront 添加元素到链表头部
func (sl *SafeList) PushFront(value interface{}) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	sl.list.PushFront(value)
}

// PopFront 移除并返回链表头部元素
func (sl *SafeList) PopFront() (interface{}, bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.list.Len() == 0 {
		return nil, false
	}
	front := sl.list.Front()
	sl.list.Remove(front)
	return front.Value, true
}

// PopBack 移除并返回链表尾部元素
func (sl *SafeList) PopBack() (interface{}, bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.list.Len() == 0 {
		return nil, false
	}
	back := sl.list.Back()
	sl.list.Remove(back)
	return back.Value, true
}

// Len 返回链表长度
func (sl *SafeList) Len() int {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	return sl.list.Len()
}

// Iterate 遍历链表
func (sl *SafeList) Iterate(action func(value interface{})) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	for e := sl.list.Front(); e != nil; e = e.Next() {
		action(e.Value)
	}
}

// IterateBreak 遍历链表
func (sl *SafeList) IterateBreak(action func(value interface{}) bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	for e := sl.list.Front(); e != nil; e = e.Next() {
		if action(e.Value) {
			return
		}
	}
}
